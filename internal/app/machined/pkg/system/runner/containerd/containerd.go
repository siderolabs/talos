// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"syscall"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/contrib/seccomp"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// containerdRunner is a runner.Runner that runs container in containerd.
type containerdRunner struct {
	args         *runner.Args
	opts         *runner.Options
	logToConsole bool

	stop    chan struct{}
	stopped chan struct{}

	client      *containerd.Client
	ctx         context.Context //nolint:containedctx
	container   containerd.Container
	stdinCloser *StdinCloser
}

// NewRunner creates runner.Runner that runs a container in containerd.
func NewRunner(logToConsole bool, args *runner.Args, setters ...runner.Option) runner.Runner {
	r := &containerdRunner{
		args:         args,
		opts:         runner.DefaultOptions(),
		logToConsole: logToConsole,
		stop:         make(chan struct{}),
		stopped:      make(chan struct{}),
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

	return nil
}

// Close implements runner.Runner interface.
func (c *containerdRunner) Close() error {
	if c.container != nil {
		err := c.container.Delete(c.ctx, containerd.WithSnapshotCleanup)
		if err != nil {
			return err
		}
	}

	if c.client == nil {
		return nil
	}

	return c.client.Close()
}

// Run implements runner.Runner interface
//
//nolint:gocyclo,cyclop
func (c *containerdRunner) Run(eventSink events.Recorder) error {
	defer close(c.stopped)

	var (
		task containerd.Task
		logW io.WriteCloser
		err  error
	)

	// attempt to clean up a task if it already exists
	task, err = c.container.Task(c.ctx, nil)
	if err == nil {
		var s <-chan containerd.ExitStatus

		s, err = task.Wait(c.ctx)
		if err != nil {
			return fmt.Errorf("failed to wait for the task %q: %w", c.args.ID, err)
		}

		err = task.Kill(c.ctx, syscall.SIGKILL, containerd.WithKillAll)
		if err != nil && !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to kill the task %q: %w", c.args.ID, err)
		}

		select {
		case <-s:
		case <-c.stop:
			return nil
		}

		if _, err = task.Delete(c.ctx); err != nil {
			return fmt.Errorf("failed to clean up task %q: %w", c.args.ID, err)
		}
	}

	logW, err = c.opts.LoggingManager.ServiceLog(c.args.ID).Writer()
	if err != nil {
		return fmt.Errorf("error creating log: %w", err)
	}

	defer logW.Close() //nolint:errcheck

	var w io.Writer = logW

	if c.logToConsole {
		w = io.MultiWriter(w, log.Writer())
	}

	r, err := c.StdinReader()
	if err != nil {
		return fmt.Errorf("failed to create stdin reader: %w", err)
	}

	creator := cio.NewCreator(cio.WithStreams(r, w, w))

	// Create the task and start it.
	task, err = c.container.NewTask(c.ctx, creator)
	if err != nil {
		return fmt.Errorf("failed to create task: %q: %w", c.args.ID, err)
	}

	if r != nil {
		// See https://github.com/containerd/containerd/issues/4489.
		go c.stdinCloser.WaitAndClose(c.ctx, task)
	}

	defer task.Delete(c.ctx) //nolint:errcheck

	if err = task.Start(c.ctx); err != nil {
		return fmt.Errorf("failed to start task: %q: %w", c.args.ID, err)
	}

	eventSink(events.StateRunning, "Started task %s (PID %d) for container %s", task.ID(), task.Pid(), c.container.ID())

	statusC, err := task.Wait(c.ctx)
	if err != nil {
		return fmt.Errorf("failed waiting for task: %q: %w", c.args.ID, err)
	}

	select {
	case status := <-statusC:
		code := status.ExitCode()
		if code != 0 {
			return fmt.Errorf("task %q failed: exit code %d", c.args.ID, code)
		}

		return nil
	case <-c.stop:
		// graceful stop the task
		eventSink(
			events.StateStopping,
			"Sending SIGTERM to task %s (PID %d, container %s)",
			task.ID(),
			task.Pid(),
			c.container.ID(),
		)

		if err = task.Kill(c.ctx, syscall.SIGTERM, containerd.WithKillAll); err != nil {
			return fmt.Errorf("error sending SIGTERM: %w", err)
		}
	}

	select {
	case <-statusC:
		// stopped process exited
		return nil
	case <-time.After(c.opts.GracefulShutdownTimeout):
		// kill the process
		eventSink(
			events.StateStopping,
			"Sending SIGKILL to task %s (PID %d, container %s)",
			task.ID(),
			task.Pid(),
			c.container.ID(),
		)

		if err = task.Kill(c.ctx, syscall.SIGKILL, containerd.WithKillAll); err != nil {
			return fmt.Errorf("error sending SIGKILL: %w", err)
		}
	}

	<-statusC

	return logW.Close()
}

// Stop implements runner.Runner interface.
func (c *containerdRunner) Stop() error {
	close(c.stop)

	<-c.stopped

	c.stop = make(chan struct{})
	c.stopped = make(chan struct{})

	return nil
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
			oci.WithImageConfig(image),
		)
	}

	specOpts = append(
		specOpts,
		oci.WithProcessArgs(c.args.ProcessArgs...),
		oci.WithEnv(c.opts.Env),
		oci.WithHostHostsFile,
		oci.WithHostResolvconf,
		oci.WithNoNewPrivileges,
	)

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
