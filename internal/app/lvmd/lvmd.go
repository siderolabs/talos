// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package lvmd implements machine.LVMService.
package lvmd

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/lvm"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// Service implements machine.LVMService.
type Service struct {
	machine.UnimplementedLVMServiceServer

	controller runtime.Controller
	logger     *zap.Logger

	lvmOnce sync.Once
	lvm     *lvm.LVM
	lvmErr  error
}

// NewService creates a new LVMService.
func NewService(controller runtime.Controller, logger *zap.Logger) *Service {
	return &Service{
		controller: controller,
		logger:     logger.With(zap.String("service", "lvmd")),
	}
}

// lvmInstance lazily initializes and returns a shared *lvm.LVM.
func (svc *Service) lvmInstance() (*lvm.LVM, error) {
	svc.lvmOnce.Do(func() {
		svc.lvm, svc.lvmErr = lvm.New()
	})

	return svc.lvm, svc.lvmErr
}

// authorize enforces the same role policy as machine.BlockDeviceWipe: Admin
// role is required unless the node is in maintenance mode.
func (svc *Service) authorize(ctx context.Context) error {
	roles := authz.GetRoles(ctx)
	inMaintenance := !svc.controller.Runtime().ConfigCompleteForBoot()

	if !inMaintenance && !roles.Includes(role.Admin) {
		return authz.ErrNotAuthorized
	}

	return nil
}

// lvmStatus maps an lvm-package sentinel to a gRPC status. The underlying
// command stderr is intentionally not surfaced — only well-known sentinels
// are reported with structured codes; everything else collapses to Internal
// with a generic message so operators do not see flags or hints that
// talosctl does not implement (e.g. `--force twice`).
func lvmStatus(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, lvm.ErrNotFound):
		return status.Error(codes.NotFound, lvm.ErrNotFound.Error())
	case errors.Is(err, lvm.ErrInUse):
		return status.Error(codes.FailedPrecondition, lvm.ErrInUse.Error())
	case errors.Is(err, lvm.ErrDevicePartitioned):
		return status.Error(codes.FailedPrecondition, lvm.ErrDevicePartitioned.Error())
	case errors.Is(err, lvm.ErrExists):
		return status.Error(codes.AlreadyExists, lvm.ErrExists.Error())
	case errors.Is(err, lvm.ErrNotEmpty):
		return status.Error(codes.FailedPrecondition, lvm.ErrNotEmpty.Error())
	case errors.Is(err, lvm.ErrOpen):
		return status.Error(codes.FailedPrecondition, lvm.ErrOpen.Error())
	case errors.Is(err, lvm.ErrInvalidCommand):
		return status.Error(codes.Internal, lvm.ErrInvalidCommand.Error())
	case errors.Is(err, lvm.ErrInitFailed):
		return status.Error(codes.FailedPrecondition, lvm.ErrInitFailed.Error())
	default:
		return status.Error(codes.Internal, "lvm operation failed")
	}
}

// invokeRemove is the shared skeleton for the three remove RPCs: authorize,
// validate, lazily init the LVM instance, then run the per-RPC action with
// structured logging and error normalization.
func (svc *Service) invokeRemove(ctx context.Context, op string, fields []zap.Field, validate func() error, action func(*lvm.LVM) error) (*emptypb.Empty, error) {
	if err := svc.authorize(ctx); err != nil {
		return nil, err
	}

	if err := validate(); err != nil {
		return nil, err
	}

	lvmInst, err := svc.lvmInstance()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to initialize LVM: %v", err)
	}

	svc.logger.Info("removing LVM resource", append([]zap.Field{zap.String("op", op)}, fields...)...)

	if err := action(lvmInst); err != nil {
		svc.logFailure(op, fields, err)

		return nil, lvmStatus(err)
	}

	return &emptypb.Empty{}, nil
}

// logFailure emits a structured warning capturing the sentinel error AND the
// raw lvm exit code / stderr (when present) so operators can diagnose the
// failure from machined logs without those details leaking back to the API
// client through the gRPC status message.
func (svc *Service) logFailure(op string, fields []zap.Field, err error) {
	all := make([]zap.Field, 0, len(fields)+4)
	all = append(all, zap.String("op", op))
	all = append(all, fields...)
	all = append(all, zap.Error(err))

	if exec, ok := errors.AsType[*lvm.ExecError](err); ok {
		all = append(
			all,
			zap.Int("lvm_exit_code", exec.ExitCode),
			zap.ByteString("lvm_stderr", exec.Stderr),
		)
	}

	svc.logger.Error("lvm remove failed", all...)
}

// LogicalVolumeRemove removes an LVM logical volume.
func (svc *Service) LogicalVolumeRemove(ctx context.Context, req *machine.LVMServiceLogicalVolumeRemoveRequest) (*emptypb.Empty, error) {
	vg, lv := req.GetVolumeGroup(), req.GetLogicalVolume()

	return svc.invokeRemove(
		ctx, "lvremove",
		[]zap.Field{zap.String("vg", vg), zap.String("lv", lv)},
		func() error {
			if vg == "" || lv == "" {
				return status.Error(codes.InvalidArgument, "volume_group and logical_volume must be set")
			}

			return nil
		},
		func(l *lvm.LVM) error { return l.LVRemove(ctx, vg, lv) },
	)
}

// VolumeGroupRemove removes an LVM volume group.
//
// Cascades to every LV in the group: vgremove --yes calls lvremove_single
// per LV before dropping the VG metadata. See lvm.VGRemove for details.
func (svc *Service) VolumeGroupRemove(ctx context.Context, req *machine.LVMServiceVolumeGroupRemoveRequest) (*emptypb.Empty, error) {
	vg := req.GetVolumeGroup()

	return svc.invokeRemove(
		ctx, "vgremove",
		[]zap.Field{zap.String("vg", vg)},
		func() error {
			if vg == "" {
				return status.Error(codes.InvalidArgument, "volume_group must be set")
			}

			return nil
		},
		func(l *lvm.LVM) error { return l.VGRemove(ctx, vg) },
	)
}

// PhysicalVolumeRemove wipes the LVM label and metadata from a block device.
func (svc *Service) PhysicalVolumeRemove(ctx context.Context, req *machine.LVMServicePhysicalVolumeRemoveRequest) (*emptypb.Empty, error) {
	device := req.GetDevice()

	return svc.invokeRemove(
		ctx, "pvremove",
		[]zap.Field{zap.String("device", device)},
		func() error {
			if device == "" {
				return status.Error(codes.InvalidArgument, "device must be set")
			}

			return nil
		},
		func(l *lvm.LVM) error { return l.PVRemove(ctx, device) },
	)
}
