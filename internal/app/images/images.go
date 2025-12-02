// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package images implements machine.ImageService.
package images

import (
	"context"
	"errors"
	"io"

	"github.com/containerd/containerd/errdefs"
	containerdapi "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/platforms"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/talos/internal/app/internal/ctrhelper"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/containers/image"
	"github.com/siderolabs/talos/internal/pkg/containers/image/progress"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

// Service implements machine.ImageService.
type Service struct {
	machine.UnimplementedImageServiceServer

	controller runtime.Controller
}

// NewService creates a new ImageService.
func NewService(controller runtime.Controller) *Service {
	return &Service{
		controller: controller,
	}
}

// List images in the containerd.
func (svc *Service) List(req *machine.ImageServiceListRequest, srv grpc.ServerStreamingServer[machine.ImageServiceListResponse]) error {
	ctx, _, client, err := ctrhelper.ContainerdInstanceHelper(srv.Context(), req.GetContainerd())
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer client.Close()

	images, err := client.ImageService().List(ctx)
	if err != nil {
		return err
	}

	for _, image := range images {
		item := &machine.ImageServiceListResponse{
			Name:      image.Name,
			Digest:    image.Target.Digest.String(),
			CreatedAt: timestamppb.New(image.CreatedAt),
		}

		size, err := image.Size(ctx, client.ContentStore(), platforms.Default())
		if err == nil {
			item.Size = size
		}

		if err = srv.Send(item); err != nil {
			return err
		}
	}

	return nil
}

// Pull an image into the containerd.
func (svc *Service) Pull(req *machine.ImageServicePullRequest, srv grpc.ServerStreamingServer[machine.ImageServicePullResponse]) error {
	ctx, _, client, err := ctrhelper.ContainerdInstanceHelper(srv.Context(), req.GetContainerd())
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer client.Close()

	img, err := image.Pull(ctx,
		cri.RegistryBuilder(svc.controller.Runtime().State().V1Alpha2().Resources()),
		client,
		req.GetImageRef(),
		image.WithSkipIfAlreadyPulled(),
		image.WithMaxNotFoundRetries(0), // return an error immediately if the image is not found
		image.WithProgressReporter(image.NewSimpleProgressReporter(func(lpp progress.LayerPullProgress) {
			srv.Send(&machine.ImageServicePullResponse{ //nolint:errcheck
				Response: &machine.ImageServicePullResponse_PullProgress{
					PullProgress: &machine.ImageServicePullProgress{
						LayerId: lpp.LayerID,
						Progress: &machine.ImageServicePullLayerProgress{
							Status:  machine.ImageServicePullLayerProgress_Status(lpp.Status),
							Elapsed: durationpb.New(lpp.Elapsed),
							Offset:  lpp.Offset,
							Total:   lpp.Total,
						},
					},
				},
			})
		})),
	)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return status.Errorf(codes.NotFound, "error pulling image: %s", err)
		}

		return err
	}

	return srv.Send(&machine.ImageServicePullResponse{
		Response: &machine.ImageServicePullResponse_Name{
			Name: img.Name(),
		},
	})
}

// Import an image from a stream (tarball).
//
//nolint:gocyclo
func (svc *Service) Import(srv grpc.ClientStreamingServer[machine.ImageServiceImportRequest, machine.ImageServiceImportResponse]) error {
	msg, err := srv.Recv()
	if err != nil {
		return err
	}

	req := msg.GetContainerd()
	if req == nil {
		return status.Errorf(codes.InvalidArgument, "containerd instance is required")
	}

	ctx, _, client, err := ctrhelper.ContainerdInstanceHelper(srv.Context(), req)
	if err != nil {
		return err
	}

	defer client.Close() //nolint:errcheck

	r, w := io.Pipe()

	go func() {
		defer w.Close() //nolint:errcheck

		for {
			msg, err := srv.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}

				w.CloseWithError(err)

				return
			}

			chunk := msg.GetImageChunk()
			if chunk == nil {
				w.CloseWithError(errors.New("no image chunk"))

				return
			}

			if _, err := w.Write(chunk.GetBytes()); err != nil {
				w.CloseWithError(err)

				return
			}
		}
	}()

	images, err := client.Import(ctx, r)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to import image: %v", err)
	}

	r.Close() //nolint:errcheck

	if len(images) == 0 {
		return status.Errorf(codes.InvalidArgument, "no images imported from archive")
	}

	imageName := images[0].Name
	for _, img := range images {
		image := containerdapi.NewImage(client, img)

		err = image.Unpack(ctx, "")
		if err != nil {
			return status.Errorf(codes.Internal, "failed to unpack image %s: %v", img.Name, err)
		}
	}

	img, err := client.GetImage(ctx, imageName)
	if err != nil {
		if errdefs.IsNotFound(err) {
			created, err := client.ImageService().Create(ctx, images[0])
			if err != nil {
				return status.Errorf(codes.Internal, "failed to create image: %v", err)
			}

			imageName = created.Name

			img, err = client.GetImage(ctx, imageName)
			if err != nil {
				return status.Errorf(codes.Internal, "failed to get image: %v", err)
			}
		}
	}

	return srv.SendAndClose(&machine.ImageServiceImportResponse{
		Name: img.Name(),
	})
}

// Remove an image from the containerd.
func (svc *Service) Remove(ctx context.Context, req *machine.ImageServiceRemoveRequest) (*emptypb.Empty, error) {
	ctx, _, client, err := ctrhelper.ContainerdInstanceHelper(ctx, req.GetContainerd())
	if err != nil {
		return nil, err
	}

	//nolint:errcheck
	defer client.Close()

	err = client.ImageService().Delete(ctx, req.ImageRef, images.SynchronousDelete())
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "image %s not found", req.ImageRef)
		}

		return nil, status.Errorf(codes.Internal, "failed to remove image %s: %v", req.ImageRef, err)
	}

	return &emptypb.Empty{}, nil
}
