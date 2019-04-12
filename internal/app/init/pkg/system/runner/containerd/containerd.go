/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package containerd

import (
	"context"
	"log"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/pkg/userdata"
)

// containerdRunner is a runner.Runner that runs container in containerd
type containerdRunner struct {
	data *userdata.UserData
	args *runner.Args
	opts *runner.Options

	stop    chan struct{}
	stopped chan struct{}
}

// errStopped is used internally to signal that task was stopped
var errStopped = errors.New("stopped")

// NewRunner creates runner.Runner that runs a container in containerd
func NewRunner(data *userdata.UserData, args *runner.Args, setters ...runner.Option) runner.Runner {
	r := &containerdRunner{
		data:    data,
		args:    args,
		opts:    runner.DefaultOptions(),
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}

	for _, setter := range setters {
		setter(r.opts)
	}

	return r
}

// Run implements the Runner interface.
// nolint: gocyclo
func (c *containerdRunner) Run() error {
	defer close(c.stopped)

	// Wait for the containerd socket.
	_, err := conditions.WaitForFileToExist(defaults.DefaultAddress)()
	if err != nil {
		return err
	}

	// Create the containerd client.

	ctx := namespaces.WithNamespace(context.Background(), c.opts.Namespace)
	client, err := containerd.New(defaults.DefaultAddress)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer client.Close()

	image, err := client.GetImage(ctx, c.opts.ContainerImage)
	if err != nil {
		return err
	}

	// Create the container.

	specOpts := c.newOCISpecOpts(image)
	containerOpts := c.newContainerOpts(image, specOpts)
	container, err := client.NewContainer(
		ctx,
		c.args.ID,
		containerOpts...,
	)
	if err != nil {
		return errors.Wrapf(err, "failed to create container %q", c.args.ID)
	}
	defer container.Delete(ctx, containerd.WithSnapshotCleanup) // nolint: errcheck

	// Manage task lifecycle
	switch c.opts.Type {
	case runner.Once:
		err = c.runOnce(ctx, container)
		if err == errStopped {
			err = nil
		}
		return err
	case runner.Forever:
		for {
			err = c.runOnce(ctx, container)
			if err == errStopped {
				return nil
			}
			if err != nil {
				log.Printf("error running %v, going to restart forever: %s", c.args.ID, err)
			}

			select {
			case <-c.stop:
				return nil
			case <-time.After(c.opts.RestartInterval):
			}
		}
	default:
		panic("unsupported runner type")
	}

}

func (c *containerdRunner) runOnce(ctx context.Context, container containerd.Container) error {
	// Create the task and start it.
	task, err := container.NewTask(ctx, cio.LogFile(c.logPath()))
	if err != nil {
		return errors.Wrapf(err, "failed to create task: %q", c.args.ID)
	}
	defer task.Delete(ctx) // nolint: errcheck

	if err = task.Start(ctx); err != nil {
		return errors.Wrapf(err, "failed to start task: %q", c.args.ID)
	}

	statusC, err := task.Wait(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed waiting for task: %q", c.args.ID)
	}

	select {
	case status := <-statusC:
		code := status.ExitCode()
		if code != 0 {
			return errors.Errorf("task %q failed: exit code %d", c.args.ID, code)
		}
		return nil
	case <-c.stop:
		// graceful stop the task
		log.Printf("sending SIGTERM to %v", c.args.ID)

		// nolint: errcheck
		_ = task.Kill(ctx, syscall.SIGTERM, containerd.WithKillAll)
	}

	select {
	case <-statusC:
		// stopped process exited
		return errStopped
	case <-time.After(c.opts.GracefulShutdownTimeout):
		// kill the process
		log.Printf("sending SIGKILL to %v", c.args.ID)

		// nolint: errcheck
		_ = task.Kill(ctx, syscall.SIGKILL, containerd.WithKillAll)
	}

	<-statusC
	return errStopped
}

// Stop implements runner.Runner interface
func (c *containerdRunner) Stop() error {
	close(c.stop)

	<-c.stopped

	return nil
}

func (c *containerdRunner) newContainerOpts(image containerd.Image, specOpts []oci.SpecOpts) []containerd.NewContainerOpts {
	containerOpts := []containerd.NewContainerOpts{
		containerd.WithImage(image),
		containerd.WithNewSnapshot(c.args.ID, image),
		containerd.WithNewSpec(specOpts...),
	}
	containerOpts = append(containerOpts, c.opts.ContainerOpts...)

	return containerOpts
}

func (c *containerdRunner) newOCISpecOpts(image oci.Image) []oci.SpecOpts {
	specOpts := []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithProcessArgs(c.args.ProcessArgs...),
		oci.WithEnv(c.opts.Env),
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithHostNamespace(specs.PIDNamespace),
		oci.WithHostHostsFile,
		oci.WithHostResolvconf,
		oci.WithPrivileged,
	}
	specOpts = append(specOpts, c.opts.OCISpecOpts...)

	return specOpts
}

func (c *containerdRunner) logPath() string {
	return filepath.Join(c.opts.LogPath, c.args.ID+".log")
}
