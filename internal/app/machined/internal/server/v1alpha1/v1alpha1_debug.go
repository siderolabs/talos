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
	"sync"
	"time"

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
	"github.com/siderolabs/talos/internal/pkg/containers/image"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

type imageCache struct {
	ctrdClient *containerdapi.Client

	mu         sync.Mutex
	images     map[containerdapi.Image]chan struct{}
	containers map[containerdapi.Container]chan struct{}
}

const debugContainerImageTTL = 5 * time.Second

func (ic *imageCache) initClientIfNil() {
	if ic.ctrdClient != nil {
		return
	}

	client, err := containerdapi.New(constants.SystemContainerdAddress,
		containerdapi.WithDefaultNamespace(constants.SystemContainerdNamespace))
	if err != nil {
		log.Printf("failed to connect to system containerd: %s", err)
	}

	ic.ctrdClient = client
}

func (ic *imageCache) addImage(img containerdapi.Image) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	usedC := make(chan struct{})

	go func() {
		timer := time.NewTimer(debugContainerImageTTL)
		select {
		case <-timer.C:
			log.Printf("debug container image TTL expired, deleting image %s", img.Name())

			ic.initClientIfNil()

			err := ic.ctrdClient.ImageService().Delete(context.Background(), img.Name(), images.SynchronousDelete())
			if err != nil {
				log.Printf("failed to delete image %s: %v", img.Name(), err)
			}
		case <-usedC:
			log.Printf("debug container image %s marked as used, skipping deletion", img.Name())
		}
	}()

	ic.images[img] = usedC
}

// TODO: client does image pull or image import RPC request
// then a debug container create RPC
// then a debug container run RPC (but these two might be the same)
// TODO: I guess there is already garbage collection done for
// images in the system namespace? maybe a tag should be added,
// so we can be more aggressive in deleting debug images.

func (ic *imageCache) addContainer(ctr containerdapi.Container) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	usedC := make(chan struct{})

	go func() {
		timer := time.NewTimer(debugContainerImageTTL)
		select {
		case <-timer.C:
			log.Printf("debug container image TTL expired, deleting container %s", ctr.ID())

			ic.initClientIfNil()

			ctr, err := ic.ctrdClient.LoadContainer(context.Background(), ctr.ID())
			if err != nil {
				log.Printf("failed to load container %s: %v", ctr.ID(), err)
			}

			err = ctr.Delete(context.Background(), containerdapi.WithSnapshotCleanup)
			if err != nil {
				log.Printf("failed to delete container %s: %v", ctr.ID(), err)
			}
		case <-usedC:
			log.Printf("debug container image %s marked as used, skipping deletion", ctr.ID())
		}
	}()

	ic.containers[ctr] = usedC
}

func (ic *imageCache) markCtrUsed(ctrID string) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	var (
		usedCtr containerdapi.Container
		usedC   chan struct{}
	)

	for ctr, c := range ic.containers {
		if ctr.ID() == ctrID {
			usedCtr = ctr
			usedC = c
		}
	}

	close(usedC)
	delete(ic.containers, usedCtr)
}

func (ic *imageCache) markImageUsed(imgName string) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	var (
		usedImage containerdapi.Image
		usedC     chan struct{}
	)

	for img, c := range ic.images {
		if img.Name() == imgName {
			usedImage = img
			usedC = c
		}
	}

	close(usedC)
	delete(ic.images, usedImage)
}

var ic = &imageCache{
	mu:         sync.Mutex{},
	images:     map[containerdapi.Image]chan struct{}{},
	containers: map[containerdapi.Container]chan struct{}{},
}

// DebugContainerCreate implements the machine.MachineServer interface.
func (s *Server) DebugContainerCreate(srv machine.MachineService_DebugContainerCreateServer) error { //nolint:gocyclo
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

	ic.addImage(img)

	ctr, err := createDebugContainer(ctx, client, img, spec)
	if err != nil {
		return err
	}

	ic.addContainer(ctr)

	return srv.Send(&machine.DebugContainerCreateResponse{
		Response: &machine.DebugContainerCreateResponse_ContainerId{
			ContainerId: ctr.ID(),
		},
	})
}

func pullImageByRef(ctx context.Context, client *containerdapi.Client, imageRef string, srv machine.MachineService_DebugContainerCreateServer) (containerdapi.Image, error) {
	updateFn := func(layerProgress *machine.ImagePullProgress) {
		if err := srv.Send(&machine.DebugContainerCreateResponse{
			Response: &machine.DebugContainerCreateResponse_PullProgress{
				PullProgress: layerProgress,
			},
		}); err != nil {
			log.Printf("debug container: failed to send pull progress: %s", err.Error())
		}
	}
	pp := image.NewPullProgress(
		client.ContentStore(),
		client.SnapshotService("overlayfs"),
		updateFn,
	)

	opts := []containerdapi.RemoteOpt{
		containerdapi.WithPlatformMatcher(platforms.Default()),
		containerdapi.WithPullUnpack,
		containerdapi.WithImageHandler(images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			if images.IsLayerType(desc.MediaType) {
				pp.Add(desc)
			}

			return nil, nil
		})),
		containerdapi.WithImageHandlerWrapper(snapshotters.AppendInfoHandlerWrapper(imageRef)),
	}

	var (
		img containerdapi.Image
		err error
	)

	finishProgress := pp.ShowProgress(ctx)
	defer finishProgress()

	img, err = client.Pull(ctx, imageRef, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to pull image: %v", err)
	}

	return img, nil
}

//nolint:gocyclo
func importImageStream(ctx context.Context,
	client *containerdapi.Client,
	srv machine.MachineService_DebugContainerCreateServer,
) (containerdapi.Image, error) {
	r, w := io.Pipe()

	go func() {
		defer w.Close() //nolint:errcheck

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

func generateContainerID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}

	return fmt.Sprintf("debug-%s", hex.EncodeToString(b)), nil
}

// DebugContainerRun implements the machine.MachineServer interface.
func (s *Server) DebugContainerRun(srv machine.MachineService_DebugContainerRunServer) error { //nolint:gocyclo
	ctx := srv.Context()

	client, err := containerdapi.New(constants.SystemContainerdAddress,
		containerdapi.WithDefaultNamespace(constants.SystemContainerdNamespace))
	if err != nil {
		return fmt.Errorf("failed to connect to system containerd: %s", err)
	}
	defer client.Close() //nolint:errcheck

	req, err := srv.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to receive container ID: %v", err)
	}

	containerID := req.GetContainerId()
	if containerID == "" {
		return status.Errorf(codes.InvalidArgument, "expected container ID")
	}

	ctr, err := client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container: %v", err)
	}

	ic.markCtrUsed(containerID)

	defer func() {
		cleanupErr := ctr.Delete(context.Background(), containerdapi.WithSnapshotCleanup)
		if cleanupErr != nil {
			log.Printf("debug container: failed to delete container %s: %s", containerID, cleanupErr.Error())
		}

		log.Printf("debug container: container %s deleted", containerID)
	}()

	ctrImage, err := ctr.Image(ctx)
	if err != nil {
		return err
	}

	ic.markImageUsed(ctrImage.Name())

	defer func() {
		err := client.ImageService().Delete(context.Background(),
			ctrImage.Name(),
			images.SynchronousDelete())
		if err != nil {
			log.Printf("debug container: failed to delete image %s: %s", ctrImage.Name(), err.Error())
		}

		log.Printf("debug container: image %s deleted", ctrImage.Name())
	}()

	return runAndAttachContainer(ctx, srv, ctr)
}

func runAndAttachContainer(
	ctx context.Context,
	srv machine.MachineService_DebugContainerRunServer,
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

	grpcStreamer.stream(statusC, task)

	return nil
}
