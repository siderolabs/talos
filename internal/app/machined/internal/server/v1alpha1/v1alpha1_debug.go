// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"

	containerdapi "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/containerd/v2/pkg/snapshotters"
	"github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/internal/pkg/capability"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// DebugContainer implements the machine.MachineServer interface.
// It receives a container image as an OCI tar archive stream, imports
// it into containerd, and runs it with debug privileges.
func (s *Server) DebugContainer(srv machine.MachineService_DebugContainerServer) error { //nolint:gocyclo
	ctx := srv.Context()

	client, err := containerdapi.New(constants.SystemContainerdAddress,
		containerdapi.WithDefaultNamespace(constants.SystemContainerdNamespace))
	if err != nil {
		return fmt.Errorf("failed to connect to system containerd: %s", err)
	}
	defer client.Close() //nolint:errcheck

	l, err := client.LeasesService().Create(ctx,
		leases.WithRandomID(),
	)
	if err != nil {
		return fmt.Errorf("failed to create lease: %v", err)
	}

	defer func() {
		if err := client.LeasesService().Delete(context.Background(), l, leases.SynchronousDelete); err != nil {
			log.Printf("failed to delete lease %s: %v", l.ID, err)
		}
	}()

	ctx = leases.WithLease(ctx, l.ID)

	specReq, err := srv.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to receive spec: %v", err)
	}

	spec := specReq.GetSpec()
	if spec == nil {
		return status.Errorf(codes.InvalidArgument, "expected debug container spec")
	}

	log.Printf("debug container request received: ref=%s args=%v", spec.ImageRef, spec.Args)

	var img containerdapi.Image
	if spec.GetImageRef() != "" {
		img, err = pullImageByRef(ctx, client, spec.GetImageRef(), srv)
		if err != nil {
			return err
		}
	} else {
		img, err = importImageStream(ctx, client, srv)
		if err != nil {
			return err
		}
	}

	defer func() {
		err = client.ImageService().Delete(context.Background(), img.Name(), images.SynchronousDelete())
		if err != nil {
			log.Printf("failed to delete image %s: %v", img.Name(), err)
		}
	}()

	ctr, err := createDebugContainer(ctx, client, img, spec)
	if err != nil {
		return err
	}

	defer func() {
		err := ctr.Delete(context.Background(), containerdapi.WithSnapshotCleanup)
		if err != nil {
			log.Printf("failed to delete debug container: %s", err.Error())
		}
	}()

	return runAndAttachContainer(ctx, srv, ctr)
}

func pullImageByRef(ctx context.Context, client *containerdapi.Client, imageRef string, srv machine.MachineService_DebugContainerServer) (containerdapi.Image, error) {
	pp := newPullProgress(srv,
		client.ContentStore(),
		client.SnapshotService("overlayfs"))

	opts := []containerdapi.RemoteOpt{
		containerdapi.WithPlatformMatcher(platforms.Default()),
		containerdapi.WithPullUnpack,
		containerdapi.WithImageHandler(images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			if images.IsLayerType(desc.MediaType) {
				pp.add(desc)
			}

			return nil, nil
		})),
		containerdapi.WithImageHandlerWrapper(snapshotters.AppendInfoHandlerWrapper(imageRef)),
	}

	var (
		img containerdapi.Image
		err error
	)

	finishProgress := pp.showProgress(ctx)
	defer finishProgress()

	img, err = client.Pull(ctx, imageRef, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to pull image: %v", err)
	}

	return img, nil
}

func importImageStream(ctx context.Context,
	client *containerdapi.Client,
	srv machine.MachineService_DebugContainerServer,
) (containerdapi.Image, error) {
	r, w := io.Pipe()

	go func() {
		for {
			msg, err := srv.Recv()
			if err != nil {
				log.Printf("import image: receive error: %s", err.Error())

				return
			}

			chunk := msg.GetImageChunk()
			if chunk == nil || len(chunk.GetBytes()) == 0 {
				return
			}

			if _, err := w.Write(chunk.GetBytes()); err != nil {
				log.Printf("import image: write error: %s", err.Error())
			}
		}
	}()

	images, err := client.Import(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to import image: %v", err)
	}

	if len(images) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no images imported from archive")
	}

	imageName := images[0].Name
	for _, img := range images {
		image := containerdapi.NewImage(client, img)

		err = image.Unpack(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("failed to unpack image %s: %v", img.Name, err)
		}
	}

	img, err := client.GetImage(ctx, imageName)
	if err != nil {
		if errdefs.IsNotFound(err) {
			created, err := client.ImageService().Create(ctx, images[0])
			if err != nil {
				return nil, err
			}

			imageName = created.Name

			img, err = client.GetImage(ctx, imageName)
			if err != nil {
				return nil, err
			}
		}
	}

	return img, nil
}

func createDebugContainer(
	ctx context.Context,
	client *containerdapi.Client,
	image containerdapi.Image,
	spec *machine.DebugContainerSpec,
) (containerdapi.Container, error) {
	ociOpts := []oci.SpecOpts{
		oci.WithDefaultSpec(),
		oci.WithDefaultUnixDevices,
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithHostNamespace(specs.PIDNamespace),
		oci.WithHostNamespace(specs.IPCNamespace),
		oci.WithTTY,
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
		oci.WithSelinuxLabel(""),
		oci.WithApparmorProfile(""),
		oci.WithSeccompUnconfined,
		oci.WithImageConfig(image),
	}

	// # mount -t tmpfs none /sys/kernel/debug/
	// # cd sys/kernel/tracing/^C
	//
	// # mkdir -p /sys/kernel/debug/tracing
	// # mount --bind /host/sys/kernel/tracing /sys/kernel/debug/tracing
	// # pwru --output-tuple 'host 1.1.1.1'
	// 2025/12/16 15:21:06 Attaching kprobes (via kprobe)...

	// TODO(laurazard): extension service
	// kprobe type network debugging
	// (packet where are you cillium/pwru)

	if len(spec.Args) > 0 {
		ociOpts = append(ociOpts, oci.WithProcessArgs(spec.Args...))
	}

	if len(spec.Env) > 0 {
		envVars := []string{}
		for k, v := range spec.Env {
			envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
		}

		ociOpts = append(ociOpts, oci.WithEnv(envVars))
	}

	containerID, err := generateContainerID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate container ID: %v", err)
	}

	container, err := client.NewContainer(ctx, containerID,
		containerdapi.WithImage(image),
		containerdapi.WithNewSnapshot(containerID+"-snapshot", image),
		containerdapi.WithNewSpec(ociOpts...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %v", err)
	}

	return container, nil
}

func runAndAttachContainer(
	ctx context.Context,
	srv machine.MachineService_DebugContainerServer,
	ctr containerdapi.Container,
) error {
	grpcStreamer, stdinR, stdoutW := newGrpcStreamWriter(srv)
	stdin := &containerd.StdinCloser{
		Stdin:  stdinR,
		Closer: make(chan struct{}),
	}

	cIo := cio.NewCreator(cio.WithStreams(stdin, stdoutW, stdoutW), cio.WithTerminal)

	task, err := ctr.NewTask(ctx, cIo)
	if err != nil {
		return fmt.Errorf("failed to create task: %v", err)
	}

	defer func() {
		_, err := task.Delete(context.Background(), containerdapi.WithProcessKill)
		if err != nil {
			log.Printf("debug container: failed to delete task: %s", err.Error())
		}
	}()

	go stdin.WaitAndClose(context.Background(), task)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := task.Start(ctx); err != nil {
		return fmt.Errorf("failed to start task: %v", err)
	}

	statusC, err := task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait for task: %v", err)
	}

	err = srv.Send(&machine.DebugContainerResponse{
		Resp: &machine.DebugContainerResponse_ContainerId{
			ContainerId: task.ID(),
		},
	})
	if err != nil {
		return err
	}

	grpcStreamer.stream(statusC, task)

	return nil
}

func generateContainerID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}

	return fmt.Sprintf("debug-%s", hex.EncodeToString(b)), nil
}
