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
	"syscall"

	containerdapi "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// DebugContainer implements the machine.MachineServer interface.
// It receives a container image as an OCI tar archive stream, imports
// it into containerd, and runs it with debug privileges.
func (s *Server) DebugContainer(srv machine.MachineService_DebugContainerServer) error {
	defer func() {
		log.Printf("debug container: debug container session ended")
	}()

	ctx := srv.Context()

	specReq, err := srv.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to receive spec: %v", err)
	}

	spec := specReq.GetSpec()
	if spec == nil {
		return status.Errorf(codes.InvalidArgument, "expected debug container spec")
	}

	log.Printf("debug container request received: args=%v", spec.Args)

	client, err := containerdapi.New(constants.SystemContainerdAddress, containerdapi.WithDefaultNamespace(constants.SystemContainerdNamespace))
	if err != nil {
		return status.Errorf(codes.Unavailable, "error connecting to system containerd: %s", err)
	}
	defer client.Close() //nolint:errcheck

	img, err := s.receiveImageArchive(ctx, client, srv)
	if err != nil {
		return err
	}

	ctr, err := s.createDebugContainer(ctx, client, img, spec)
	if err != nil {
		return err
	}
	defer ctr.Delete(ctx, containerdapi.WithSnapshotCleanup) //nolint:errcheck

	return s.runAndAttachContainer(ctx, srv, ctr)
}

func (s *Server) receiveImageArchive(ctx context.Context,
	client *containerdapi.Client,
	srv machine.MachineService_DebugContainerServer,
) (containerdapi.Image, error) {
	ctx = namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)

	images, err := client.Import(ctx, &imageChunkReaderWrapper{srv: srv})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to import image: %v", err)
	}

	if len(images) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no images imported from archive")
	}

	imageName := images[0].Name
	for _, img := range images {
		image := containerdapi.NewImage(client, img)

		log.Printf("unpacking %s (%s)...", img.Name, img.Target.Digest)

		err = image.Unpack(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("failed to unpack image %s: %v", img.Name, err)
		}
	}

	image, err := client.GetImage(ctx, imageName)
	if err != nil {
		if errdefs.IsNotFound(err) {
			imageService := client.ImageService()

			created, err := imageService.Create(ctx, images[0])
			if err != nil {
				log.Printf("Error while creating image: %+v", err)
				client.Close() //nolint:errcheck

				return nil, err
			}

			imageName = created.Name

			image, err = client.GetImage(ctx, imageName)
			if err != nil {
				log.Printf("Error while getting image after creation: %+v", err)
				client.Close() //nolint:errcheck

				return nil, err
			}
		}
	}

	return image, nil
}

func (s *Server) createDebugContainer(
	ctx context.Context,
	client *containerdapi.Client,
	image containerdapi.Image,
	spec *machine.DebugContainerSpec,
) (containerdapi.Container, error) {
	// Build OCI spec options
	ociOpts := []oci.SpecOpts{
		oci.WithDefaultSpec(),
		oci.WithDefaultUnixDevices,
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithHostNamespace(specs.PIDNamespace),
		oci.WithHostNamespace(specs.IPCNamespace),
		oci.WithImageConfig(image),
		oci.WithTTY,
	}

	// 	ociOpts = append(ociOpts,
	// 		// oci.WithPrivileged,
	// 		oci.WithAllDevicesAllowed,
	// 		oci.WithHostDevices,
	// 	)

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
		return nil, status.Errorf(codes.Internal, "failed to generate container ID: %v", err)
	}

	container, err := client.NewContainer(ctx, containerID,
		containerdapi.WithImage(image),
		containerdapi.WithNewSnapshot(containerID+"-snapshot", image),
		containerdapi.WithNewSpec(ociOpts...),
	)
	if err != nil {
		log.Printf("debug container: failed to create container: %v", err)

		ctrInfo, err := container.Info(ctx) //nolint:errcheck
		if err == nil {
			client.SnapshotService(ctrInfo.Snapshotter).Remove(ctx, containerID+"-snapshot") //nolint:errcheck
		}

		return nil, fmt.Errorf("failed to create container: %v", err)
	}

	return container, nil
}

func (s *Server) runAndAttachContainer(
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
		log.Printf("debug container: failed to create task: %v", err)

		return status.Errorf(codes.Internal, "failed to create task: %v", err)
	}

	go stdin.WaitAndClose(context.Background(), task)

	defer task.Delete(ctx) //nolint:errcheck

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := task.Start(ctx); err != nil {
		return status.Errorf(codes.Internal, "failed to start task: %v", err)
	}

	statusC, err := task.Wait(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to wait for task: %v", err)
	}

	grpcStreamer.streamWithTask(statusC, task)

	return nil
}

// imageChunkReaderWrapper is a plain io.Reader wrapper
// around the gRPC stream that we can pass to containerd.
//
// Since we don't control the size of the incoming chunks,
// (or the `Read()`s), we hold onto the last received chunk
// in case len(chunk) > read size.
//
// TODO(laurazard): this could:
//   - be faster, if we just loaded the entire image into
//     memory instead of reading from the client piecemeal
//   - be simpler, by just passing a buffer into containerd
//     and `io.Copy`ing into it.
type imageChunkReaderWrapper struct {
	srv machine.MachineService_DebugContainerServer

	// buffered image chunk data - size of incoming chunks may
	// be larger than reads, and we don't want to lose data in
	// between reads.
	currentChunk []byte
	// Read position in currentChunk
	offset int
}

func (i *imageChunkReaderWrapper) Read(b []byte) (int, error) {
	return i.readImageChunk(b)
}

func (i *imageChunkReaderWrapper) readImageChunk(b []byte) (int, error) {
	if len(i.currentChunk)-i.offset == 0 {
		err := i.loadNewChunk()
		if err != nil {
			return 0, err
		}
	}

	n := copy(b, i.currentChunk[i.offset:])
	i.offset += n

	return n, nil
}

func (i *imageChunkReaderWrapper) loadNewChunk() error {
	msg, err := i.srv.Recv()
	if err != nil {
		return err
	}

	chunk := msg.GetImageChunk()
	if chunk == nil || len(chunk.GetBytes()) == 0 {
		return io.EOF
	}

	i.currentChunk = chunk.GetBytes()
	i.offset = 0

	return nil
}

func newGrpcStreamWriter(srv machine.MachineService_DebugContainerServer) (
	*grpcStdioStreamer,
	io.Reader,
	io.Writer,
) {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	return &grpcStdioStreamer{
		srv:     srv,
		stdinW:  stdinW,
		stdoutR: stdoutR,
		stdoutW: stdoutW,
	}, stdinR, stdoutW
}

// TODO: out of band communication
type grpcStdioStreamer struct {
	srv machine.MachineService_DebugContainerServer

	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter
}

func (r *grpcStdioStreamer) streamWithTask(statusC <-chan containerdapi.ExitStatus, task containerdapi.Task) {
	recvLoopC := r.recvLoopWithTask(task)
	sendLoopC := r.sendLoop()

	select {
	case ec := <-statusC:
		// closing r.stdoutW causes the sendLoop, which s
		// blocking on r.stdoutR.Read(), to get an EOF and exit
		r.stdoutW.Close() //nolint:errcheck
		<-sendLoopC

		// then, sending the exit code back to the client makes
		// the client disconnect, causing the recvLoop which is
		// hanging on srv.Recv() to exit
		r.srv.Send(&machine.DebugContainerResponse{ //nolint:errcheck
			Resp: &machine.DebugContainerResponse_ExitCode{
				ExitCode: int32(ec.ExitCode()),
			},
		})
		<-recvLoopC

		return

	case <-recvLoopC:
	}

	// the client has disconnected, so we close r.stdinW
	// causing the container to exit
	r.stdinW.Close() //nolint:errcheck
	// t.CloseIO(context.Background(), containerdapi.WithStdinCloser) //nolint:errcheck
	<-statusC

	// close stdoutW so that sendLoop will get an EOF
	// and exit
	r.stdoutW.Close() //nolint:errcheck
	<-sendLoopC
}

func (r *grpcStdioStreamer) recvLoopWithTask(task containerdapi.Task) chan struct{} {
	done := make(chan struct{})

	go func() {
	LOOP:
		for {
			msg, err := r.srv.Recv()
			if err != nil {
				if status.Code(err) != codes.Canceled {
					log.Printf("debug container: recv error: %s", err.Error())
				}

				break LOOP
			}

			switch msg.Request.(type) {
			case *machine.DebugContainerRequest_StdinData:
				if stdinData := msg.GetStdinData(); stdinData != nil {
					_, err := r.stdinW.Write(stdinData)
					if err != nil {
						if err != io.EOF {
							log.Printf("debug container: stdin write error: %s", err.Error())
						}

						break LOOP
					}
				}

			case *machine.DebugContainerRequest_TermResize:
				log.Printf("debug container: received terminal resize request")
				width := msg.GetTermResize().Width
				height := msg.GetTermResize().Height
				task.Resize(context.Background(), uint32(width), uint32(height)) //nolint:errcheck

			case *machine.DebugContainerRequest_Signal:
				signalNum := msg.GetSignal()
				log.Printf("debug container: received signal %d, forwarding to task", signalNum)

				if err := task.Kill(context.Background(), syscall.Signal(signalNum)); err != nil {
					log.Printf("debug container: failed to forward signal to task: %v", err)
				}

			default:
				log.Printf("debug container: unknown request type")
			}
		}

		done <- struct{}{}
	}()

	return done
}

func (r *grpcStdioStreamer) sendLoop() chan struct{} {
	done := make(chan struct{})

	go func() {
		b := make([]byte, 512)

	LOOP:
		for {
			n, err := r.stdoutR.Read(b)
			if err != nil {
				if err != io.EOF {
					log.Printf("debug container: stdout read error: %s", err.Error())
				}

				break LOOP
			}

			err = r.srv.Send(&machine.DebugContainerResponse{
				Resp: &machine.DebugContainerResponse_StdoutData{
					StdoutData: b[:n],
				},
			})
			if err != nil {
				if status.Code(err) != codes.Canceled {
					log.Printf("debug container: send error: %s", err.Error())
				}

				break LOOP
			}
		}

		done <- struct{}{}
	}()

	return done
}

// generateContainerID generates a unique container ID with the debug- prefix.
func generateContainerID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}

	return fmt.Sprintf("debug-%s", hex.EncodeToString(b)), nil
}
