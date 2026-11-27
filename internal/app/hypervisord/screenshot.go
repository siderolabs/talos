// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisord

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// WithScreenshotConnector enables screenshots. The connector must verify live ownership,
// UUID and running state, honor ctx, and return the encoded image with its MIME type.
func WithScreenshotConnector(capture func(context.Context, domain.Domain) ([]byte, string, error)) ServiceOption {
	return func(s *Service) { s.screenshot = capture }
}

// Screenshot implements machine.HypervisorServiceServer.
//
// Only virtual machines with VNC enabled have a video device to capture.
func (s *Service) Screenshot(ctx context.Context, req *machine.VirtualMachineScreenshotRequest) (*machine.VirtualMachineScreenshotResponse, error) {
	if s.screenshot == nil {
		return nil, status.Error(codes.Unimplemented, "screenshot connector is not configured")
	}

	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "virtual machine name is required")
	}

	identity, err := s.attachmentIdentity(ctx, req.GetName(), true)
	if err != nil {
		return nil, err
	}

	image, mime, err := s.screenshot(ctx, identity)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return nil, status.FromContextError(err).Err()
		case errors.Is(err, domain.ErrScreenshotTooLarge):
			return nil, status.Error(codes.ResourceExhausted, err.Error())
		default:
			return nil, domainError(err, "take virtual machine screenshot")
		}
	}

	return &machine.VirtualMachineScreenshotResponse{
		Image:    image,
		MimeType: mime,
	}, nil
}
