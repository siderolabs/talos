// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package debug implements machine.DebugService.
package debug

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"path/filepath"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/internal/ctrhelper"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/internal/pkg/capability"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Service implements machine.DebugService.
type Service struct {
	machine.UnimplementedDebugServiceServer
}

// ContainerRun implements machine.DebugService.ContainerRun.
func (s *Service) ContainerRun(srv grpc.BidiStreamingServer[machine.DebugContainerRunRequest, machine.DebugContainerRunResponse]) error { //nolint:gocyclo
	ctx := srv.Context()

	// 1. get the debug container spec
	specReq, err := srv.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to receive spec: %v", err)
	}

	spec := specReq.GetSpec()
	if spec == nil {
		return status.Errorf(codes.InvalidArgument, "expected debug container spec")
	}

	if spec.GetProfile() != machine.DebugContainerRunRequestSpec_PROFILE_PRIVILEGED {
		return status.Errorf(codes.InvalidArgument, "unsupported debug container profile: %s", spec.GetProfile())
	}

	log.Printf("debug container request received: image=%s args=%v env=%v profile=%s", spec.GetImageName(), spec.GetArgs(), spec.GetEnv(), spec.GetProfile())

	// 2. connect to containerd with a lease
	ctx, detachedContext, c8dClient, err := ctrhelper.ContainerdInstanceHelper(ctx, spec.GetContainerd())
	if err != nil {
		return err
	}
	defer c8dClient.Close() //nolint:errcheck

	l, err := c8dClient.LeasesService().Create(ctx,
		leases.WithRandomID(),
	)
	if err != nil {
		return fmt.Errorf("failed to create lease: %v", err)
	}

	defer func() {
		if err := c8dClient.LeasesService().Delete(detachedContext, l, leases.SynchronousDelete); err != nil {
			log.Printf("failed to delete lease %s: %v", l.ID, err)
		}
	}()

	ctx = leases.WithLease(ctx, l.ID)

	img, err := c8dClient.GetImage(ctx, spec.ImageName)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return status.Errorf(codes.NotFound, "image %s not found: %v", spec.ImageName, err)
		}

		return err
	}

	// 3. create the debug container
	containerID, err := generateContainerID()
	if err != nil {
		return fmt.Errorf("failed to generate container ID: %v", err)
	}

	// create the root cgroup to populate resources as needed
	_, err = cgroup.CreateCgroup(constants.CgroupSystemDebug)
	if err != nil {
		return fmt.Errorf("failed to create debug cgroup: %v", err)
	}

	cgroupPath := filepath.Join(constants.CgroupSystemDebug, containerID)

	// create a cgroup for this container instance, and clean it up afterwards
	cg, err := cgroup.CreateCgroup(cgroupPath)
	if err != nil {
		return fmt.Errorf("failed to create cgroup for debug container: %v", err)
	}

	defer func() {
		cg.Delete() // nolint: errcheck
	}()

	ctr, err := createDebugContainer(ctx, c8dClient, containerID, img, spec, cgroupPath)
	if err != nil {
		return err
	}

	log.Printf("debug container: container %s created", ctr.ID())

	defer func() {
		cleanupErr := ctr.Delete(detachedContext, client.WithSnapshotCleanup)
		if cleanupErr != nil {
			log.Printf("debug container: failed to delete container %s: %s", ctr.ID(), cleanupErr.Error())
		}

		log.Printf("debug container: container %s deleted", ctr.ID())
	}()

	// 4. run and attach to the debug container
	return runAndAttachContainer(ctx, detachedContext, spec, srv, ctr)
}

func createDebugContainer(
	ctx context.Context,
	c8dClient *client.Client,
	containerID string,
	image client.Image,
	spec *machine.DebugContainerRunRequestSpec,
	cgroupPath string,
) (client.Container, error) {
	ociOpts := []oci.SpecOpts{
		oci.WithDefaultSpec(),
		oci.WithDefaultUnixDevices,
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithHostNamespace(specs.PIDNamespace),
		oci.WithHostNamespace(specs.IPCNamespace),
		oci.WithHostDevices,
		oci.WithAllDevicesAllowed,
		oci.WithHostHostsFile,
		oci.WithWriteableSysfs,
		oci.WithCapabilities(capability.AllGrantableCapabilities()),
		oci.WithHostResolvconf,
		oci.WithMounts([]specs.Mount{
			// mount host / under /host
			{
				Destination: "/host",
				Type:        "bind",
				Source:      "/",
				Options:     []string{"rbind", "rw"},
			},
			{
				Destination: "/sys",
				Type:        "bind",
				Source:      "/sys",
				Options:     []string{"rbind", "rw"},
			},
		}),
		oci.WithSelinuxLabel(""), // TODO: consider implementing a specific policy for debug containers
		oci.WithApparmorProfile(""),
		oci.WithSeccompUnconfined,
		oci.WithImageConfig(image),
		oci.WithCgroup(cgroupPath),
	}

	if spec.GetTty() {
		ociOpts = append(ociOpts, oci.WithTTY)
	}

	if len(spec.Args) > 0 {
		ociOpts = append(ociOpts, oci.WithProcessArgs(spec.Args...))
	}

	if len(spec.Env) > 0 {
		envVars := make([]string, 0, len(spec.Env))

		for k, v := range spec.Env {
			envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
		}

		ociOpts = append(ociOpts, oci.WithEnv(envVars))
	}

	container, err := c8dClient.NewContainer(ctx, containerID,
		client.WithImage(image),
		client.WithNewSnapshot(containerID+"-snapshot", image),
		client.WithNewSpec(ociOpts...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %v", err)
	}

	return container, nil
}

func runAndAttachContainer(
	ctx context.Context,
	detachedContext context.Context,
	spec *machine.DebugContainerRunRequestSpec,
	srv grpc.BidiStreamingServer[machine.DebugContainerRunRequest, machine.DebugContainerRunResponse],
	ctr client.Container,
) error {
	grpcStreamer, stdinR, stdoutW := newGrpcStreamWriter(srv)
	stdin := &containerd.StdinCloser{
		Stdin:  stdinR,
		Closer: make(chan struct{}),
	}

	cIoOpts := []cio.Opt{
		cio.WithStreams(stdin, stdoutW, stdoutW),
	}

	if spec.GetTty() {
		cIoOpts = append(cIoOpts, cio.WithTerminal)
	}

	cIo := cio.NewCreator(cIoOpts...)

	task, err := ctr.NewTask(ctx, cIo)
	if err != nil {
		return fmt.Errorf("failed to create task: %v", err)
	}

	defer func() {
		_, err := task.Delete(detachedContext, client.WithProcessKill)
		if err != nil && !errdefs.IsNotFound(err) {
			log.Printf("debug container: failed to delete task: %s", err.Error())
		}
	}()

	go stdin.WaitAndClose(ctx, task)

	if err := task.Start(ctx); err != nil {
		return fmt.Errorf("failed to start task: %v", err)
	}

	statusC, err := task.Wait(detachedContext)
	if err != nil {
		return fmt.Errorf("failed to wait for task: %v", err)
	}

	return grpcStreamer.stream(ctx, statusC, task)
}

func generateContainerID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}

	return fmt.Sprintf("debug-%s", hex.EncodeToString(b)), nil
}
