// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"syscall"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/contrib/seccomp"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/internal/lastlog"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// containerdRunner is a runner.Runner that runs container in containerd.
type containerdRunner struct {
	args         *runner.Args
	opts         *runner.Options
	logToConsole bool

	client      *containerd.Client
	ctx         context.Context //nolint:containedctx
	container   containerd.Container
	stdinCloser *StdinCloser
}

const (
	taskWaitRetryInterval = time.Second

	// killWaitTimeout bounds the wait for a task to be reaped after SIGKILL.
	killWaitTimeout = 30 * time.Second
)

var errTaskWaitClosed = errors.New("task wait stream closed without an exit status")

// NewRunner creates runner.Runner that runs a container in containerd.
func NewRunner(logToConsole bool, args *runner.Args, setters ...runner.Option) runner.Runner {
	r := &containerdRunner{
		args:         args,
		opts:         runner.DefaultOptions(),
		logToConsole: logToConsole,
	}

	for _, setter := range setters {
		setter(r.opts)
	}

	return r
}

// Open implements the Runner interface.
func (c *containerdRunner) Open() error {
	// Create the containerd client.
	var err error

	c.ctx = namespaces.WithNamespace(context.Background(), c.opts.Namespace)

	c.client, err = containerd.New(c.opts.ContainerdAddress)
	if err != nil {
		return err
	}

	// Callers do not Close a runner whose Open failed, so anything that fails past this point has to
	// release the client itself or it leaks along with its gRPC connection.
	opened := false

	defer func() {
		if !opened {
			c.client.Close() //nolint:errcheck

			c.client = nil
		}
	}()

	var image containerd.Image

	if c.opts.ContainerImage != "" {
		image, err = c.client.GetImage(c.ctx, c.opts.ContainerImage)
		if err != nil {
			return err
		}
	}

	// See if there's previous container/snapshot to clean up
	var oldcontainer containerd.Container

	if oldcontainer, err = c.client.LoadContainer(c.ctx, c.args.ID); err == nil {
		if err = c.deleteStaleTask(oldcontainer); err != nil {
			return fmt.Errorf("error deleting old task instance: %w", err)
		}

		if err = oldcontainer.Delete(c.ctx, containerd.WithSnapshotCleanup); err != nil {
			return fmt.Errorf("error deleting old container instance: %w", err)
		}
	}

	if err = c.client.SnapshotService("").Remove(c.ctx, c.args.ID); err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("error cleaning up stale snapshot: %w", err)
	}

	// Create the container.
	specOpts := c.newOCISpecOpts(image)
	containerOpts := c.newContainerOpts(image, specOpts)

	c.container, err = c.client.NewContainer(
		c.ctx,
		c.args.ID,
		containerOpts...,
	)
	if err != nil {
		return fmt.Errorf("failed to create container %q: %w", c.args.ID, err)
	}

	opened = true

	return nil
}

// Close implements runner.Runner interface.
//
// The client is closed whatever happens to the container: a task that would not die after SIGKILL is
// still holding its container, so deleting it fails, and returning there would leak the client.
func (c *containerdRunner) Close() error {
	var errs error

	if c.container != nil {
		if err := c.container.Delete(c.ctx, containerd.WithSnapshotCleanup); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	if c.client != nil {
		errs = errors.Join(errs, c.client.Close())
	}

	return errs
}

// Run implements runner.Runner interface.
//
// Canceling ctx starts the graceful stop; every containerd call keeps using c.ctx, which is not
// derived from it, because the kill and delete have to run after the cancellation rather than be
// canceled by it.
//
// A non-zero exit is reported both ways: in Status.ExitCode, and as an error, because the restart
// policies decide purely on the error and a task that exits non-zero must not read as success.
//
//nolint:gocyclo,cyclop
func (c *containerdRunner) Run(ctx context.Context, eventSink events.Recorder, onStart runner.OnStart) (runner.Status, error) {
	var (
		task   containerd.Task
		logW   io.WriteCloser
		status runner.Status
		err    error
	)

	// attempt to clean up a task if it already exists
	task, err = c.container.Task(c.ctx, nil)
	if err == nil {
		var s <-chan containerd.ExitStatus

		s, err = task.Wait(c.ctx)
		if err != nil {
			return status, fmt.Errorf("failed to wait for the task %q: %w", c.args.ID, err)
		}

		err = task.Kill(c.ctx, syscall.SIGKILL, containerd.WithKillAll)
		if err != nil && !errdefs.IsNotFound(err) {
			return status, fmt.Errorf("failed to kill the task %q: %w", c.args.ID, err)
		}

		select {
		case <-s:
		case <-ctx.Done():
			return status, nil
		}

		if _, err = task.Delete(c.ctx); err != nil {
			return status, fmt.Errorf("failed to clean up task %q: %w", c.args.ID, err)
		}
	}

	logID := c.opts.LogID
	if logID == "" {
		logID = c.args.ID
	}

	logW, err = c.opts.LoggingManager.ServiceLog(logID).Writer()
	if err != nil {
		return status, fmt.Errorf("error creating log: %w", err)
	}

	cg, err := c.createCgroup()
	if err != nil {
		return status, fmt.Errorf("error creating cgroup: %w", err)
	}

	// If the task is not cleaned up by containerd or another error
	// happens during the lifecycle, remove the cgroup before exiting
	// if one still exists
	defer func() {
		err := cg.Delete()
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			eventSink(events.StateStopping, "Failed to remove cgroup for %s, %s", c, err)
		}
	}()

	defer logW.Close() //nolint:errcheck

	var (
		lastLog lastlog.Writer
		w       = io.MultiWriter(logW, &lastLog)
	)

	if c.logToConsole {
		w = io.MultiWriter(w, log.Writer())
	}

	r, err := c.StdinReader()
	if err != nil {
		return status, fmt.Errorf("failed to create stdin reader: %w", err)
	}

	creator := cio.NewCreator(cio.WithStreams(r, w, w))

	// Create the task and start it.
	task, err = c.container.NewTask(c.ctx, creator)
	if err != nil {
		return status, fmt.Errorf("failed to create task: %q: %w", c.args.ID, err)
	}

	if r != nil {
		// See https://github.com/containerd/containerd/issues/4489.
		go c.stdinCloser.WaitAndClose(c.ctx, task)
	}

	defer task.Delete(c.ctx) //nolint:errcheck

	if err = task.Start(c.ctx); err != nil {
		return status, fmt.Errorf("failed to start task: %q: %w", c.args.ID, err)
	}

	eventSink(events.StateRunning, "Started task %s (PID %d) for container %s", task.ID(), task.Pid(), c.container.ID())

	status.Started = true

	if onStart != nil {
		onStart(int32(task.Pid())) //nolint:gosec
	}

	recovering := false

taskWaitLoop:
	for {
		statusC, waitErr := task.Wait(c.ctx)
		if waitErr != nil {
			return status, fmt.Errorf("failed waiting for task %q: %w", c.args.ID, waitErr)
		}

		select {
		case taskStatus, ok := <-statusC:
			retry, statusErr := retryTaskWait(taskStatus, ok)
			if statusErr != nil {
				return status, fmt.Errorf("failed waiting for task %q: %w", c.args.ID, statusErr)
			}

			if !retry {
				code := taskStatus.ExitCode()
				status.ExitCode = int(code) //nolint:gosec

				if code != 0 {
					return status, fmt.Errorf("task %q failed: exit code %d (last log %q)", c.args.ID, code, lastLog.GetLastLog())
				}

				return status, nil
			}

			if !recovering {
				log.Printf("task wait stream interrupted, waiting for containerd: %v", taskStatus.Error())
			}

			recovering = true

			select {
			case <-ctx.Done():
				break taskWaitLoop
			case <-time.After(taskWaitRetryInterval):
			}
		case <-ctx.Done():
			break taskWaitLoop
		}
	}

	// graceful stop the task
	eventSink(
		events.StateStopping,
		"Sending SIGTERM to task %s (PID %d, container %s)",
		task.ID(),
		task.Pid(),
		c.container.ID(),
	)

	gracefulDeadline := time.Now().Add(c.opts.GracefulShutdownTimeout)

	// not delivering SIGTERM before the deadline is not fatal: the task gets SIGKILL below
	if err = killTask(c.ctx, task, syscall.SIGTERM, gracefulDeadline); err != nil && !isContainerdUnavailable(err) && !isDeadlineExceeded(err) {
		return status, fmt.Errorf("error sending SIGTERM: %w", err)
	}

	exited, err := waitTaskExit(c.ctx, task, gracefulDeadline)
	if err != nil {
		return status, fmt.Errorf("failed waiting for task after SIGTERM: %w", err)
	}

	if exited {
		return status, nil
	}

	eventSink(
		events.StateStopping,
		"Sending SIGKILL to task %s (PID %d, container %s)",
		task.ID(),
		task.Pid(),
		c.container.ID(),
	)

	// Bounded, because SIGKILL is not the end of every story: a task stuck in uninterruptible sleep
	// never reaps, and waiting for it forever would hold up whatever is driving the stop.
	killDeadline := time.Now().Add(killWaitTimeout)

	if err = killTask(c.ctx, task, syscall.SIGKILL, killDeadline); err != nil {
		return status, fmt.Errorf("error sending SIGKILL: %w", err)
	}

	exited, err = waitTaskExit(c.ctx, task, killDeadline)
	if err != nil {
		return status, fmt.Errorf("failed waiting for task after SIGKILL: %w", err)
	}

	if !exited {
		return status, fmt.Errorf("task %q did not exit after SIGKILL", c.args.ID)
	}

	return status, logW.Close()
}

// deleteStaleTask kills and deletes the task of a container left behind by a previous instance of the service.
//
// The previous instance can lose track of its task, e.g. when containerd restarts while the task is being
// stopped: the container then can't be deleted while its task runs, and the service could never start again.
func (c *containerdRunner) deleteStaleTask(container containerd.Container) error {
	task, err := container.Task(c.ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}

		return err
	}

	ctx, cancel := context.WithTimeout(c.ctx, killWaitTimeout)
	defer cancel()

	if _, err = task.Delete(ctx, containerd.WithProcessKill); err != nil && !errdefs.IsNotFound(err) {
		return err
	}

	return nil
}

// killTask signals the task, retrying while containerd is unavailable (e.g. restarting) until the deadline.
func killTask(ctx context.Context, task containerd.Task, signal syscall.Signal, deadline time.Time) error {
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	retrying := false

	for {
		err := task.Kill(ctx, signal, containerd.WithKillAll)
		if err == nil || isTaskNotFound(err) {
			return nil
		}

		if !isContainerdUnavailable(err) {
			return err
		}

		if !retrying {
			log.Printf("containerd unavailable while sending %s to task %q, retrying: %v", unix.SignalName(signal), task.ID(), err)
		}

		retrying = true

		select {
		case <-ctx.Done():
			return err
		case <-time.After(taskWaitRetryInterval):
		}
	}
}

// waitTaskExit waits for the task to exit until the deadline, and reports whether it did.
//
// An interrupted wait stream (containerd restarting) is not an exit: the task keeps running under its shim,
// so the wait is re-established instead of reporting the task as stopped.
func waitTaskExit(ctx context.Context, task containerd.Task, deadline time.Time) (bool, error) {
	timeout := time.NewTimer(time.Until(deadline))
	defer timeout.Stop()

	interrupted := false

	for {
		statusC, waitErr := task.Wait(ctx)
		if waitErr == nil {
			select {
			case taskStatus, ok := <-statusC:
				retry, statusErr := retryTaskWait(taskStatus, ok)
				if statusErr == nil && !retry {
					return true, nil
				}

				waitErr = statusErr
				if retry {
					waitErr = taskStatus.Error()
				}
			case <-timeout.C:
				return false, nil
			}
		}

		if isTaskNotFound(waitErr) {
			return true, nil
		}

		if !isContainerdUnavailable(waitErr) {
			return false, waitErr
		}

		if !interrupted {
			log.Printf("task %q wait stream interrupted while stopping, waiting for containerd: %v", task.ID(), waitErr)
		}

		interrupted = true

		select {
		case <-timeout.C:
			return false, nil
		case <-time.After(taskWaitRetryInterval):
		}
	}
}

func retryTaskWait(status containerd.ExitStatus, ok bool) (bool, error) {
	if !ok {
		return false, errTaskWaitClosed
	}

	if err := status.Error(); err != nil {
		if isContainerdUnavailable(err) {
			return true, nil
		}

		return false, err
	}

	return false, nil
}

func isContainerdUnavailable(err error) bool {
	return errdefs.IsUnavailable(err) || status.Code(err) == codes.Unavailable
}

func isDeadlineExceeded(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded
}

func isTaskNotFound(err error) bool {
	return errdefs.IsNotFound(err) || status.Code(err) == codes.NotFound
}

// createCgroup creates the cgroup for the task, with caller-supplied limits when there are any.
func (c *containerdRunner) createCgroup() (cgroup.CommonCgroup, error) {
	if c.opts.CgroupResources != nil {
		return cgroup.CreateCgroupWithResources(c.opts.CgroupPath, c.opts.CgroupResources)
	}

	return cgroup.CreateCgroup(c.opts.CgroupPath)
}

func (c *containerdRunner) newContainerOpts(
	image containerd.Image,
	specOpts []oci.SpecOpts,
) []containerd.NewContainerOpts {
	var containerOpts []containerd.NewContainerOpts

	if image != nil {
		containerOpts = append(
			containerOpts,
			containerd.WithImage(image),
			containerd.WithNewSnapshot(c.args.ID, image),
		)
	}

	containerOpts = append(
		containerOpts,
		containerd.WithNewSpec(specOpts...),
	)

	containerOpts = append(
		containerOpts,
		c.opts.ContainerOpts...,
	)

	return containerOpts
}

func (c *containerdRunner) newOCISpecOpts(image oci.Image) []oci.SpecOpts {
	var specOpts []oci.SpecOpts

	if image != nil {
		specOpts = append(
			specOpts,
			WithImageConfigStripped(image),
		)
	}

	specOpts = append(
		specOpts,
		oci.WithProcessArgs(c.args.ProcessArgs...),
		oci.WithEnv(c.opts.Env),
		oci.WithNoNewPrivileges,
	)

	if c.opts.HostNetworkFiles {
		specOpts = append(specOpts,
			oci.WithHostHostsFile,
			oci.WithHostResolvconf,
		)
	}

	if c.opts.OOMScoreAdj != 0 {
		specOpts = append(
			specOpts,
			WithOOMScoreAdj(c.opts.OOMScoreAdj),
		)
	}

	if c.opts.CgroupPath != "" {
		specOpts = append(
			specOpts,
			oci.WithCgroup(cgroup.Path(c.opts.CgroupPath)),
		)
	}

	specOpts = append(
		specOpts,
		c.opts.OCISpecOpts...,
	)

	if c.opts.OverrideSeccompProfile != nil {
		specOpts = append(
			specOpts,
			WithCustomSeccompProfile(c.opts.OverrideSeccompProfile),
		)
	} else {
		specOpts = append(
			specOpts,
			seccomp.WithDefaultProfile(), // add seccomp profile last, as it depends on process capabilities
		)
	}

	if selinux.IsEnabled() {
		if c.opts.SelinuxLabel != "" {
			specOpts = append(
				specOpts,
				oci.WithSelinuxLabel(c.opts.SelinuxLabel),
			)
		} else {
			specOpts = append(
				specOpts,
				oci.WithSelinuxLabel(constants.SelinuxLabelUnconfinedSysContainer),
			)
		}
	}

	return specOpts
}

func (c *containerdRunner) String() string {
	return fmt.Sprintf("Containerd(%v)", c.args.ID)
}

func (c *containerdRunner) StdinReader() (io.Reader, error) {
	if c.opts.Stdin == nil {
		return nil, nil
	}

	if _, err := c.opts.Stdin.Seek(0, 0); err != nil {
		return nil, err
	}

	// copy the input buffer as containerd API seems to be buggy:
	//  * if the task fails to start, IO loop is not stopped properly, so after a restart there are two goroutines concurrently reading from stdin
	contents, err := io.ReadAll(c.opts.Stdin)
	if err != nil {
		return nil, err
	}

	c.stdinCloser = &StdinCloser{
		Stdin:  bytes.NewReader(contents),
		Closer: make(chan struct{}),
	}

	return c.stdinCloser, nil
}
