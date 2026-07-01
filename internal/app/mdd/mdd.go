// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package mdd implements machine.MDService.
//
// Only Destroy is wired up (exposed via "talosctl wipe md"); Create/Extend/
// Shrink are left to the embedded UnimplementedMDServiceServer until the
// bootable-mirror provisioning path lands.
package mdd

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/md"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// Service implements machine.MDService.
type Service struct {
	machine.UnimplementedMDServiceServer

	controller runtime.Controller
	logger     *zap.Logger

	mdOnce sync.Once
	md     *md.MD
	mdErr  error
}

// NewService creates a new MDService.
func NewService(controller runtime.Controller, logger *zap.Logger) *Service {
	return &Service{
		controller: controller,
		logger:     logger.With(zap.String("service", "mdd")),
	}
}

// mdInstance lazily initializes and returns a shared *md.MD.
func (svc *Service) mdInstance() (*md.MD, error) {
	svc.mdOnce.Do(func() {
		svc.md, svc.mdErr = md.New()
	})

	return svc.md, svc.mdErr
}

// authorize enforces the same role policy as the LVM and BlockDeviceWipe APIs:
// Admin role is required unless the node is in maintenance mode.
func (svc *Service) authorize(ctx context.Context) error {
	roles := authz.GetRoles(ctx)
	inMaintenance := !svc.controller.Runtime().ConfigCompleteForBoot()

	if !inMaintenance && !roles.Includes(role.Admin) {
		return authz.ErrNotAuthorized
	}

	return nil
}

// mdStatus maps an md-package sentinel to a gRPC status. The underlying mdadm
// stderr is intentionally not surfaced - only well-known sentinels are
// reported with structured codes; everything else collapses to Internal with
// a generic message.
func mdStatus(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, md.ErrNotFound):
		return status.Error(codes.NotFound, md.ErrNotFound.Error())
	case errors.Is(err, md.ErrInUse):
		return status.Error(codes.FailedPrecondition, md.ErrInUse.Error())
	case errors.Is(err, md.ErrExists):
		return status.Error(codes.AlreadyExists, md.ErrExists.Error())
	case errors.Is(err, md.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, md.ErrInvalidArgument.Error())
	default:
		return status.Error(codes.Internal, "md operation failed")
	}
}

// logFailure emits a structured warning capturing the sentinel error AND the
// raw mdadm exit code / stderr (when present) so operators can diagnose the
// failure from machined logs without those details leaking back to the API
// client through the gRPC status message.
func (svc *Service) logFailure(op string, fields []zap.Field, err error) {
	all := make([]zap.Field, 0, len(fields)+4)
	all = append(all, zap.String("op", op))
	all = append(all, fields...)
	all = append(all, zap.Error(err))

	if exec, ok := errors.AsType[*md.ExecError](err); ok {
		all = append(
			all,
			zap.Int("mdadm_exit_code", exec.ExitCode),
			zap.ByteString("mdadm_stderr", exec.Stderr),
		)
	}

	svc.logger.Error("md operation failed", all...)
}

// Destroy stops the array and clears member superblocks.
func (svc *Service) Destroy(ctx context.Context, req *machine.MDDestroyRequest) (*emptypb.Empty, error) {
	if err := svc.authorize(ctx); err != nil {
		return nil, err
	}

	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name must be set")
	}

	mdInst, err := svc.mdInstance()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to initialize mdadm: %v", err)
	}

	fields := []zap.Field{zap.String("name", name)}

	svc.logger.Info("destroying MD array", fields...)

	if err := mdInst.Destroy(ctx, name); err != nil {
		svc.logFailure("destroy", fields, err)

		return nil, mdStatus(err)
	}

	return &emptypb.Empty{}, nil
}
