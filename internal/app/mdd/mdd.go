// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package mdd implements machine.MDService.
//
// Destroy (exposed via "talosctl wipe md") and Add (exposed via "talosctl md
// add") are wired up; Create/Extend/Shrink are left to the embedded
// UnimplementedMDServiceServer until the bootable-mirror provisioning path
// lands.
package mdd

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/siderolabs/go-blockdevice/v2/blkid"
	blockdev "github.com/siderolabs/go-blockdevice/v2/block"
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

// lockTimeout bounds the wait for udev or the volume discovery to release a disk being added.
const lockTimeout = 10 * time.Second

// Service implements machine.MDService.
type Service struct {
	machine.UnimplementedMDServiceServer

	controller runtime.Controller
	logger     *zap.Logger

	mdOnce sync.Once
	md     *md.MD
	mdErr  error

	// addMu serializes Add, so that two calls cannot both see the array degraded and leave the
	// second disk as a spare.
	addMu sync.Mutex
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

	device := req.GetDevice()
	if device == "" {
		return nil, status.Error(codes.InvalidArgument, "device must be set")
	}

	if !strings.HasPrefix(device, "/dev/") {
		return nil, status.Error(codes.InvalidArgument, "device must be a full /dev path")
	}

	mdInst, err := svc.mdInstance()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to initialize mdadm: %v", err)
	}

	fields := []zap.Field{zap.String("device", device)}

	svc.logger.Info("destroying MD array", fields...)

	if err := mdInst.Destroy(ctx, device); err != nil {
		svc.logFailure("destroy", fields, err)

		return nil, mdStatus(err)
	}

	return &emptypb.Empty{}, nil
}

// Add adds a disk to a degraded raid1 array to restore its redundancy.
func (svc *Service) Add(ctx context.Context, req *machine.MDAddRequest) (*emptypb.Empty, error) {
	if err := svc.authorize(ctx); err != nil {
		return nil, err
	}

	device := req.GetDevice()

	arrayDev, memberDev, err := resolveAddPaths(device, req.GetMember())
	if err != nil {
		return nil, err
	}

	mdInst, err := svc.mdInstance()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to initialize mdadm: %v", err)
	}

	fields := []zap.Field{zap.String("device", arrayDev), zap.String("member", memberDev)}

	svc.addMu.Lock()
	defer svc.addMu.Unlock()

	info, err := arrayInfo(ctx, mdInst, arrayDev)
	if err != nil {
		// the array may also have been stopped since its path was resolved
		if errors.Is(err, md.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
			return nil, status.Errorf(codes.NotFound, "%q is not an active MD array", device)
		}

		svc.logFailure("add", fields, err)

		return nil, mdStatus(err)
	}

	if err = ValidateAdd(ctx, svc.controller.Runtime().State().V1Alpha2().Resources(), info, filepath.Base(memberDev)); err != nil {
		return nil, err
	}

	if err = svc.addBlankDisk(ctx, mdInst, arrayDev, memberDev, fields); err != nil {
		return nil, err
	}

	// udev only probes a disk when it appears: without a change event the new member keeps no
	// raid member signature in the udev database, so unplugging it later would never run
	// `mdadm -If` to drop it from the array.
	if err = os.WriteFile(filepath.Join("/sys/class/block", filepath.Base(memberDev), "uevent"), []byte("change\n"), 0o644); err != nil {
		svc.logger.Warn("failed to trigger a change event for the new MD member", append(fields, zap.Error(err))...)
	}

	return &emptypb.Empty{}, nil
}

// addBlankDisk adds the disk to the array, as long as it is still blank once locked.
func (svc *Service) addBlankDisk(ctx context.Context, mdInst *md.MD, arrayDev, memberDev string, fields []zap.Field) error {
	unlock, err := lockBlankDisk(ctx, memberDev)
	if err != nil {
		return err
	}

	defer unlock()

	svc.logger.Info("adding disk to MD array", fields...)

	if err = mdInst.Add(ctx, arrayDev, memberDev); err != nil {
		svc.logFailure("add", fields, err)

		return mdStatus(err)
	}

	return nil
}

// resolveAddPaths validates the paths of an Add request and resolves them to /dev nodes.
func resolveAddPaths(device, member string) (arrayDev, memberDev string, err error) {
	for _, arg := range []struct{ field, path string }{{"device", device}, {"member", member}} {
		if arg.path == "" {
			return "", "", status.Errorf(codes.InvalidArgument, "%s must be set", arg.field)
		}

		if !strings.HasPrefix(arg.path, "/dev/") {
			return "", "", status.Errorf(codes.InvalidArgument, "%s must be a full /dev path", arg.field)
		}
	}

	if arrayDev, err = filepath.EvalSymlinks(device); err != nil {
		return "", "", status.Errorf(codes.NotFound, "array %q not found", device)
	}

	if memberDev, err = filepath.EvalSymlinks(member); err != nil {
		return "", "", status.Errorf(codes.NotFound, "disk %q not found", member)
	}

	// both must resolve to a device node, which is what the validation looks up by name
	for _, resolved := range []string{arrayDev, memberDev} {
		if filepath.Dir(resolved) != "/dev" {
			return "", "", status.Errorf(codes.InvalidArgument, "%q is not a device node in /dev", resolved)
		}
	}

	return arrayDev, memberDev, nil
}

// lockBlankDisk locks the disk exclusively and probes it once more.
//
// The resources ValidateAdd relies on may lag behind the disk itself: the lock keeps udev and the
// volume discovery off the disk until mdadm has written its superblock, and the probe makes sure
// nothing was written to it in the meantime. The returned func releases the lock.
func lockBlankDisk(ctx context.Context, memberDev string) (func(), error) {
	bd, err := blockdev.NewFromPath(memberDev)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "failed to open disk %q: %v", memberDev, err)
	}

	if err = bd.RetryLockWithTimeout(ctx, true, lockTimeout); err != nil {
		bd.Close() //nolint:errcheck

		return nil, status.Errorf(codes.FailedPrecondition, "disk %q is busy: %v", memberDev, err)
	}

	unlock := func() {
		bd.Unlock() //nolint:errcheck
		bd.Close()  //nolint:errcheck
	}

	info, err := blkid.Probe(bd.File(), blkid.WithSkipLocking(true))
	if err != nil {
		unlock()

		return nil, status.Errorf(codes.Internal, "failed to probe disk %q: %v", memberDev, err)
	}

	switch {
	case info.Name != "":
		unlock()

		return nil, notBlank(filepath.Base(memberDev), info.Name+" signature found")
	case len(info.Parts) > 0:
		unlock()

		return nil, notBlank(filepath.Base(memberDev), "it has partitions")
	}

	return unlock, nil
}

// arrayInfo observes the state of the array at the resolved /dev/mdN path.
func arrayInfo(ctx context.Context, mdInst *md.MD, arrayDev string) (ArrayInfo, error) {
	// fails with md.ErrNotFound unless arrayDev is an MD array
	inSync, err := md.InSyncMembers(arrayDev)
	if err != nil {
		return ArrayInfo{}, err
	}

	detail, err := mdInst.DetailDevice(ctx, arrayDev)
	if err != nil {
		return ArrayInfo{}, err
	}

	info := ArrayInfo{
		Name:          detail.Name,
		Level:         detail.Level,
		Members:       detail.Members,
		InSyncMembers: inSync,
	}

	// levels without redundancy have no degraded count nor sync action: ValidateAdd rejects them by level
	if info.Level != levelRAID1 {
		return info, nil
	}

	if info.Degraded, err = md.DegradedForDevice(arrayDev); err != nil {
		return ArrayInfo{}, err
	}

	syncAction, err := md.SyncActionForDevice(arrayDev)
	if err != nil {
		return ArrayInfo{}, err
	}

	info.SyncAction = string(syncAction)

	return info, nil
}
