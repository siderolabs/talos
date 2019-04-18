/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package containerd

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// containerdRunner is a runner.Runner that runs container in containerd
type containerdRunner struct {
	data *userdata.UserData
	args *runner.Args
	opts *runner.Options

	stop    chan struct{}
	stopped chan struct{}

	client    *containerd.Client
	ctx       context.Context
	container containerd.Container
}

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

// Open implements the Runner interface.
func (c *containerdRunner) Open() error {

	// Wait for the containerd socket.
	_, err := conditions.WaitForFileToExist(constants.ContainerdAddress)()
	if err != nil {
		return err
	}

	// Create the containerd client.

	c.ctx = namespaces.WithNamespace(context.Background(), c.opts.Namespace)
	c.client, err = containerd.New(constants.ContainerdAddress)
	if err != nil {
		return err
	}

	image, err := c.client.GetImage(c.ctx, c.opts.ContainerImage)
	if err != nil {
		return err
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
		return errors.Wrapf(err, "failed to create container %q", c.args.ID)
	}

	return nil
}

// Close implements runner.Runner interface
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
func (c *containerdRunner) Run() error {
	defer close(c.stopped)

	// Create the task and start it.
	task, err := c.container.NewTask(c.ctx, cio.LogFile(c.logPath()))
	if err != nil {
		return errors.Wrapf(err, "failed to create task: %q", c.args.ID)
	}
	defer task.Delete(c.ctx) // nolint: errcheck

	if err = task.Start(c.ctx); err != nil {
		return errors.Wrapf(err, "failed to start task: %q", c.args.ID)
	}

	statusC, err := task.Wait(c.ctx)
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
		_ = task.Kill(c.ctx, syscall.SIGTERM, containerd.WithKillAll)
	}

	select {
	case <-statusC:
		// stopped process exited
		return nil
	case <-time.After(c.opts.GracefulShutdownTimeout):
		// kill the process
		log.Printf("sending SIGKILL to %v", c.args.ID)

		// nolint: errcheck
		_ = task.Kill(c.ctx, syscall.SIGKILL, containerd.WithKillAll)
	}

	<-statusC
	return nil
}

// Stop implements runner.Runner interface
func (c *containerdRunner) Stop() error {
	close(c.stop)

	<-c.stopped

	c.stop = make(chan struct{})
	c.stopped = make(chan struct{})

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

func (c *containerdRunner) String() string {
	return fmt.Sprintf("Containerd(%v)", c.args.ID)
}
