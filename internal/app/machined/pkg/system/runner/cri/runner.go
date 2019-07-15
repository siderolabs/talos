/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cri

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/pkg/cri"
	"github.com/talos-systems/talos/pkg/userdata"
)

type criRunner struct {
	data *userdata.UserData
	args *runner.Args
	opts *runner.Options

	stop    chan struct{}
	stopped chan struct{}

	client *cri.Client

	podSandboxConfig *runtimeapi.PodSandboxConfig
	podSandboxID     string
	imageRef         string
}

// NewRunner creates runner.Runner that runs a container in a sandbox
func NewRunner(data *userdata.UserData, args *runner.Args, setters ...runner.Option) runner.Runner {
	r := &criRunner{
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

// Close implements runner.Runner interface
func (c *criRunner) Close() error {
	ctx, ctxCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer ctxCancel()

	if c.podSandboxID != "" {
		err := c.client.StopPodSandbox(ctx, c.podSandboxID)
		if err != nil {
			return err
		}

		err = c.client.RemovePodSandbox(ctx, c.podSandboxID)
		if err != nil {
			return err
		}

		c.podSandboxID = ""

	}

	if c.client == nil {
		return nil
	}

	return c.client.Close()
}

func (c *criRunner) findImage(ctx context.Context) error {
	imageInfo, err := c.client.ImageStatus(ctx, &runtimeapi.ImageSpec{
		Image: c.opts.ContainerImage,
	})
	if err != nil {
		return err
	}

	if imageInfo != nil {
		c.imageRef = imageInfo.Id
		return nil
	}

	// ListImages API at least in the containerd CRI plugin ignores the filter,
	// so instead of relying on it, request full list and filter manually
	images, err := c.client.ListImages(ctx, &runtimeapi.ImageFilter{})
	if err != nil {
		return err
	}

	for _, image := range images {
		if image.Id == c.opts.ContainerImage {
			c.imageRef = image.Id
			return nil
		}

		for _, repoTag := range image.RepoTags {
			if repoTag == c.opts.ContainerImage {
				c.imageRef = image.Id
				return nil
			}
		}
	}

	return errors.Errorf("container image %q hasn't been found", c.opts.ContainerImage)
}

// Open prepares the runner.
//
//nolint: gocyclo
func (c *criRunner) Open(upstreamCtx context.Context) error {
	// validate the basic options
	if c.opts.Namespace != "system" {
		return errors.New("namespaces not supported by CRI runner")
	}

	if c.opts.ContainerOpts != nil {
		return errors.New("containerd options not supported by CRI runner")
	}

	if c.opts.OCISpecOpts != nil {
		return errors.New("OCI spec is not supported by CRI runner")
	}

	ctx, ctxCancel := context.WithTimeout(upstreamCtx, 30*time.Second)
	defer ctxCancel()

	// Create the CRI client.
	var err error
	c.client, err = cri.NewClient("unix:"+c.opts.ContainerdAddress, 10*time.Second)
	if err != nil {
		return err
	}

	if err = c.findImage(ctx); err != nil {
		return err
	}

	// See if there's previous pod sandbox to clean up
	oldSandboxes, err := c.client.ListPodSandbox(ctx, &runtimeapi.PodSandboxFilter{
		LabelSelector: map[string]string{
			"talos.id": c.args.ID,
		},
	})
	if err != nil {
		return err
	}

	for _, oldSandbox := range oldSandboxes {
		if oldSandbox.State == runtimeapi.PodSandboxState_SANDBOX_READY {
			if err = c.client.StopPodSandbox(ctx, oldSandbox.Id); err != nil {
				return err
			}
		}

		if err = c.client.RemovePodSandbox(ctx, oldSandbox.Id); err != nil {
			return err
		}
	}

	// Create pod sandbox
	c.podSandboxConfig = &runtimeapi.PodSandboxConfig{
		Metadata: &runtimeapi.PodSandboxMetadata{
			Name:      c.args.ID,
			Uid:       c.args.ID,
			Namespace: "talos",
		},
		LogDirectory: c.opts.LogPath,
		Labels: map[string]string{
			"talos.id": c.args.ID,
		},
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			SecurityContext: &runtimeapi.LinuxSandboxSecurityContext{
				NamespaceOptions: &runtimeapi.NamespaceOption{
					Network: runtimeapi.NamespaceMode_NODE,
				},
			},
		},
	}

	// Create the pod
	c.podSandboxID, err = c.client.RunPodSandbox(ctx, c.podSandboxConfig, "")

	return err
}

// Run implements runner.Runner interface
//
// nolint: gocyclo
func (c *criRunner) Run(eventSink events.Recorder) error {
	defer close(c.stopped)

	ctx := context.Background()

	// Create container
	containerConfig := runtimeapi.ContainerConfig{
		Metadata: &runtimeapi.ContainerMetadata{
			Name: c.args.ID,
		},
		Image: &runtimeapi.ImageSpec{
			Image: c.imageRef,
		},
		Command: c.args.ProcessArgs,
		LogPath: c.args.ID + ".log",
	}

	// Create the container
	containerID, err := c.client.CreateContainer(ctx, c.podSandboxID, &containerConfig, c.podSandboxConfig)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer c.client.RemoveContainer(ctx, containerID)

	// Start the container
	err = c.client.StartContainer(ctx, containerID)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer c.client.StopContainer(context.Background(), containerID, 5)

	eventSink(events.StateRunning, "Started container %q in sandbox %q", containerID, c.podSandboxID)

	// TODO: make this polling interval configurable
	pollTicker := time.NewTicker(1 * time.Second)
	defer pollTicker.Stop()

WAIT:
	for {
		select {
		case <-c.stop:
			break WAIT
		case <-pollTicker.C:
		}

		var status *runtimeapi.ContainerStatus
		status, _, err = c.client.ContainerStatus(ctx, containerID, false)
		if err != nil {
			return err
		}

		switch status.State {
		case runtimeapi.ContainerState_CONTAINER_RUNNING:
			// ok
		case runtimeapi.ContainerState_CONTAINER_EXITED:
			if status.ExitCode == 0 {
				return nil
			}

			return errors.Errorf("container exited with code %d (%s)", status.ExitCode, status.Reason)
		default:
			return errors.Errorf("container in unexpected state (%d)", status.State)
		}
	}

	eventSink(events.StateStopping, "Stopping container %q in sandbox %q", containerID, c.podSandboxID)
	err = c.client.StopContainer(ctx, containerID, int64(c.opts.GracefulShutdownTimeout/time.Second))
	if err != nil {
		return err
	}

	return c.client.RemoveContainer(ctx, containerID)
}

// Stop implements runner.Runner interface
func (c *criRunner) Stop() error {
	close(c.stop)

	<-c.stopped

	c.stop = make(chan struct{})
	c.stopped = make(chan struct{})

	return nil
}

func (c *criRunner) String() string {
	return fmt.Sprintf("CRI(%v)", c.args.ID)
}
