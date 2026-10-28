// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package debug

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"syscall"
	"time"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/errdefs"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/internal/ctrhelper"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// execCleanupTimeout bounds removing an exec'd process once the command is done.
const execCleanupTimeout = 10 * time.Second

// ContainerExec implements machine.DebugService.ContainerExec.
//
//nolint:gocyclo,cyclop
func (s *Service) ContainerExec(srv grpc.BidiStreamingServer[machine.DebugContainerExecRequest, machine.DebugContainerExecResponse]) error {
	ctx := srv.Context()

	specReq, err := srv.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to receive spec: %v", err)
	}

	spec := specReq.GetSpec()

	switch {
	case spec == nil:
		return status.Errorf(codes.InvalidArgument, "expected exec spec")
	case spec.GetContainer() == "":
		return status.Errorf(codes.InvalidArgument, "container is required")
	case len(spec.GetArgs()) == 0:
		return status.Errorf(codes.InvalidArgument, "args are required")
	}

	name := spec.GetContainer()

	instanceID, err := s.runningInstance(ctx, name)
	if err != nil {
		return err
	}

	ctx, detachedContext, c8dClient, err := ctrhelper.ContainerdInstanceHelper(ctx, &common.ContainerdInstance{
		Driver:    common.ContainerDriver_CONTAINERD,
		Namespace: common.ContainerdNamespace_NS_TALOSCONTAINERS,
	})
	if err != nil {
		return err
	}
	defer c8dClient.Close() //nolint:errcheck

	ctr, err := c8dClient.LoadContainer(ctx, instanceID)
	if err != nil {
		return containerNotRunning(name, err)
	}

	task, err := ctr.Task(ctx, nil)
	if err != nil {
		return containerNotRunning(name, err)
	}

	ctrSpec, err := ctr.Spec(ctx)
	if err != nil {
		return fmt.Errorf("failed to get spec of container %q: %w", name, err)
	}

	if ctrSpec.Process == nil {
		return status.Errorf(codes.Internal, "container %q has no process spec", name)
	}

	// The process of the container itself, so the command runs as it does: user, environment,
	// capabilities and SELinux label. Namespaces, cgroup and mounts are the container's anyway.
	processSpec := *ctrSpec.Process
	processSpec.Args = spec.GetArgs()
	processSpec.Terminal = spec.GetTty()

	execID, err := generateExecID()
	if err != nil {
		return err
	}

	logger := s.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	logger = logger.With(zap.String("container", name), zap.String("instance", instanceID), zap.String("exec_id", execID))

	sess := newSession(execStream{srv: srv, tty: spec.GetTty()}, !spec.GetTty())
	stdin := &containerd.StdinCloser{
		Stdin:  sess.stdin(),
		Closer: make(chan struct{}),
	}

	cIoOpts := []cio.Opt{cio.WithStreams(stdin, sess.stdout(), sess.stderr())}

	if spec.GetTty() {
		cIoOpts = []cio.Opt{cio.WithStreams(stdin, sess.stdout(), nil), cio.WithTerminal}
	}

	process, err := task.Exec(ctx, execID, &processSpec, sess.ioCreator(cIoOpts...))
	if err != nil {
		sess.close()

		return containerNotRunning(name, err)
	}

	defer func() {
		// containerd waits for the stdio of the process to be copied, so it is released first.
		sess.close()

		cleanupCtx, cancel := context.WithTimeout(detachedContext, execCleanupTimeout)
		defer cancel()

		if _, deleteErr := process.Delete(cleanupCtx, client.WithProcessKill); deleteErr != nil {
			if !errdefs.IsNotFound(deleteErr) {
				logger.Warn("failed to delete container exec process", zap.Error(deleteErr))
			}

			if processIO := process.IO(); processIO != nil {
				processIO.Cancel()
				processIO.Close() //nolint:errcheck
			}
		}
	}()

	statusC, err := process.Wait(detachedContext)
	if err != nil {
		return fmt.Errorf("failed to wait for exec in container %q: %w", name, err)
	}

	go stdin.WaitAndClose(ctx, process)

	// Started on the detached context: a canceled start could leave the process running, without the
	// PID which killing it in the cleanup above relies on.
	startCtx, cancelStart := context.WithTimeout(detachedContext, execCleanupTimeout)
	defer cancelStart()

	if err = process.Start(startCtx); err != nil {
		// The command may have started regardless, and without its PID the cleanup can't kill it, so
		// it is killed by its exec ID.
		killCtx, cancelKill := context.WithTimeout(detachedContext, execCleanupTimeout)
		defer cancelKill()

		if killErr := process.Kill(killCtx, syscall.SIGKILL); killErr == nil {
			select {
			case <-statusC:
			case <-killCtx.Done():
			}
		}

		return containerNotRunning(name, err)
	}

	logger.Info("container exec started",
		zap.Strings("args", spec.GetArgs()),
		zap.Bool("tty", spec.GetTty()),
		zap.Strings("roles", authz.GetRoles(ctx).Strings()),
	)

	if err = srv.Send(&machine.DebugContainerExecResponse{
		Resp: &machine.DebugContainerExecResponse_InstanceId{InstanceId: instanceID},
	}); err != nil {
		return err
	}

	// The exit code is kept for the log, as the session only passes it on to the client.
	exitC := make(chan exitResult, 1)

	var exitCode int32

	go func() {
		result := <-processExit(statusC)

		exitCode = result.code
		exitC <- result
	}()

	err = sess.run(ctx, containerdProcess{process}, exitC, processOutputDrained(process))
	if err != nil {
		logger.Info("container exec failed", zap.Error(err))

		return fmt.Errorf("container exec: %w", err)
	}

	logger.Info("container exec finished", zap.Int32("exit_code", exitCode))

	return nil
}

// runningInstance returns the ID of the instance currently running the container.
//
// The instance can still be replaced before the command is started in it, in which case starting
// it fails the same way.
func (s *Service) runningInstance(ctx context.Context, name string) (string, error) {
	if _, err := safe.ReaderGetByID[*containers.ContainerSpec](ctx, s.Resources, name); err != nil {
		if state.IsNotFoundError(err) {
			return "", status.Errorf(codes.NotFound, "container %q not found", name)
		}

		return "", err
	}

	instanceStatus, err := containers.LatestInstanceStatus(ctx, s.Resources, name)
	if err != nil {
		return "", err
	}

	if instanceStatus == nil || instanceStatus.TypedSpec().Phase != containers.ContainerInstancePhaseRunning {
		return "", status.Errorf(codes.FailedPrecondition, "container %q is not running", name)
	}

	instanceSpec, err := containers.CurrentInstanceSpec(ctx, s.Resources, name, instanceStatus)
	if err != nil {
		return "", err
	}

	if instanceSpec == nil || instanceSpec.Metadata().Phase() != resource.PhaseRunning {
		return "", status.Errorf(codes.FailedPrecondition, "container %q is not running", name)
	}

	return instanceSpec.Metadata().ID(), nil
}

// containerNotRunning reports a container which went away before the command could be started in it.
func containerNotRunning(name string, err error) error {
	if errdefs.IsNotFound(err) {
		return status.Errorf(codes.FailedPrecondition, "container %q is not running", name)
	}

	return fmt.Errorf("failed to exec in container %q: %w", name, err)
}

func generateExecID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}

	return fmt.Sprintf("exec-%s", hex.EncodeToString(b)), nil
}

// execStream adapts ContainerExec to a session.
type execStream struct {
	srv grpc.BidiStreamingServer[machine.DebugContainerExecRequest, machine.DebugContainerExecResponse]
	tty bool
}

func (e execStream) recv() (controlMessage, error) {
	for {
		msg, err := e.srv.Recv()
		if err != nil {
			// a half-close ends stdin as well
			if err == io.EOF && e.tty {
				<-e.srv.Context().Done()
			}

			return controlMessage{}, err
		}

		// closing the stdin of a terminal hangs it up, so with a terminal stdin stays open until the command exits
		if _, ok := msg.Request.(*machine.DebugContainerExecRequest_StdinEof); ok && e.tty {
			continue
		}

		return e.controlMessage(msg)
	}
}

func (e execStream) controlMessage(msg *machine.DebugContainerExecRequest) (controlMessage, error) {
	switch msg.Request.(type) {
	case *machine.DebugContainerExecRequest_StdinData:
		return controlMessage{stdinData: msg.GetStdinData()}, nil
	case *machine.DebugContainerExecRequest_StdinEof:
		return controlMessage{stdinEOF: msg.GetStdinEof()}, nil
	case *machine.DebugContainerExecRequest_Signal:
		return controlMessage{signal: msg.GetSignal()}, nil
	case *machine.DebugContainerExecRequest_TermResize:
		return controlMessage{resize: msg.GetTermResize()}, nil
	default:
		return controlMessage{}, fmt.Errorf("unknown request type: %T", msg.Request)
	}
}

func (e execStream) sendOutput(stream outputStream, data []byte) error {
	if stream == outputStderr {
		return e.srv.Send(&machine.DebugContainerExecResponse{
			Resp: &machine.DebugContainerExecResponse_StderrData{StderrData: data},
		})
	}

	return e.srv.Send(&machine.DebugContainerExecResponse{
		Resp: &machine.DebugContainerExecResponse_StdoutData{StdoutData: data},
	})
}

func (e execStream) sendExitCode(code int32) error {
	return e.srv.Send(&machine.DebugContainerExecResponse{
		Resp: &machine.DebugContainerExecResponse_ExitCode{ExitCode: code},
	})
}
