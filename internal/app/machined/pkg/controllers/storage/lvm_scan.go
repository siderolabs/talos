// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/lvm"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// LVMScanner is the subset of internal/pkg/lvm.LVM used by LVMScanController.
// Splitting it out lets tests inject a fake without touching /sbin/lvm.
type LVMScanner interface {
	VGS(ctx context.Context) ([]lvm.VG, error)
	PVS(ctx context.Context) ([]lvm.PV, error)
	LVS(ctx context.Context) ([]lvm.LV, error)
}

// LVMScanController owns LVMVolumeGroupStatus / LVMPhysicalVolumeStatus /
// LVMLogicalVolumeStatus.
//
// Driven by LVMRefreshRequest (no poll). Runs vgs/pvs/lvs once per counter
// increment and echoes observed value into LVMRefreshStatus.
type LVMScanController struct {
	LVM LVMScanner
}

// Name implements controller.Controller interface.
func (ctrl *LVMScanController) Name() string {
	return "storage.LVMScanController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LVMScanController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: storage.NamespaceName,
			Type:      storage.LVMRefreshRequestType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LVMScanController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: storage.LVMVolumeGroupStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: storage.LVMPhysicalVolumeStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: storage.LVMLogicalVolumeStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: storage.LVMRefreshStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *LVMScanController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var lastObserved int

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		req, err := safe.ReaderGetByID[*storage.LVMRefreshRequest](ctx, r, storage.RefreshID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("get LVM refresh request: %w", err)
		}

		if req == nil || req.TypedSpec().Request == lastObserved {
			continue
		}

		observed := req.TypedSpec().Request

		if err := ctrl.scan(ctx, r, logger); err != nil {
			return err
		}

		if err := safe.WriterModify(
			ctx, r,
			storage.NewLVMRefreshStatus(storage.NamespaceName, storage.RefreshID),
			func(s *storage.LVMRefreshStatus) error {
				s.TypedSpec().Request = observed

				return nil
			},
		); err != nil {
			return fmt.Errorf("write LVM refresh status: %w", err)
		}

		lastObserved = observed
	}
}

func (ctrl *LVMScanController) scan(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	vgs, err := ctrl.LVM.VGS(ctx)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			logger.Debug("lvm binary not found; skipping LVM scan")

			return nil
		}

		return fmt.Errorf("vgs: %w", err)
	}

	pvs, err := ctrl.LVM.PVS(ctx)
	if err != nil {
		return fmt.Errorf("pvs: %w", err)
	}

	lvs, err := ctrl.LVM.LVS(ctx)
	if err != nil {
		return fmt.Errorf("lvs: %w", err)
	}

	if err := ctrl.applyVGs(ctx, r, vgs); err != nil {
		return err
	}

	if err := ctrl.applyPVs(ctx, r, pvs); err != nil {
		return err
	}

	if err := ctrl.applyLVs(ctx, r, lvs); err != nil {
		return err
	}

	return nil
}

func (ctrl *LVMScanController) applyVGs(ctx context.Context, r controller.Runtime, vgs []lvm.VG) error {
	r.StartTrackingOutputs()

	for _, vg := range vgs {
		if err := safe.WriterModify(
			ctx, r,
			storage.NewLVMVolumeGroupStatus(storage.NamespaceName, vg.Name),
			func(s *storage.LVMVolumeGroupStatus) error {
				spec := s.TypedSpec()
				spec.Name = vg.Name
				spec.UUID = vg.UUID
				spec.Format = vg.Format
				spec.Permissions = vg.Permissions
				spec.Extendable = vg.Extendable
				spec.Exported = vg.Exported
				spec.Partial = vg.Partial
				spec.AllocationPolicy = vg.AllocationPolicy
				spec.Clustered = vg.Clustered
				spec.Shared = vg.Shared
				spec.Size = vg.Size
				spec.Free = vg.Free
				spec.ExtentSize = vg.ExtentSize
				spec.ExtentCount = vg.ExtentCount
				spec.FreeExtentCount = vg.FreeExtentCount
				spec.MaxLV = vg.MaxLV
				spec.MaxPV = vg.MaxPV
				spec.LVCount = vg.LVCount
				spec.PVCount = vg.PVCount
				spec.SnapCount = vg.SnapCount
				spec.MissingPVCount = vg.MissingPVCount
				spec.SeqNo = vg.SeqNo
				spec.LockType = vg.LockType
				spec.SystemID = vg.SystemID
				spec.Tags = []string(vg.Tags)

				return nil
			},
		); err != nil {
			return fmt.Errorf("modify vg %q: %w", vg.Name, err)
		}
	}

	if err := safe.CleanupOutputs[*storage.LVMVolumeGroupStatus](ctx, r); err != nil {
		return fmt.Errorf("cleanup vg outputs: %w", err)
	}

	return nil
}

func (ctrl *LVMScanController) applyPVs(ctx context.Context, r controller.Runtime, pvs []lvm.PV) error {
	r.StartTrackingOutputs()

	for _, pv := range pvs {
		// `pvs -a` enumerates every block device on the host; rows that are
		// not actual LVM PVs come back with an empty UUID. Skip them so we
		// don't publish a resource per loop/disk-partition device.
		if pv.UUID == "" {
			continue
		}

		if err := safe.WriterModify(
			ctx, r,
			storage.NewLVMPhysicalVolumeStatus(storage.NamespaceName, pvID(pv.Device)),
			func(s *storage.LVMPhysicalVolumeStatus) error {
				spec := s.TypedSpec()
				spec.Device = pv.Device
				spec.VGName = pv.VGName
				spec.UUID = pv.UUID
				spec.Format = pv.Format
				spec.Allocatable = pv.Allocatable
				spec.Exported = pv.Exported
				spec.Missing = pv.Missing
				spec.InUse = pv.InUse
				spec.Size = pv.Size
				spec.DeviceSize = pv.DeviceSize
				spec.Free = pv.Free
				spec.Used = pv.Used
				spec.PECount = pv.PECount
				spec.PEAllocCount = pv.PEAllocCount
				spec.Major = pv.Major
				spec.Minor = pv.Minor
				spec.Tags = []string(pv.Tags)

				return nil
			},
		); err != nil {
			return fmt.Errorf("modify pv %q: %w", pv.Device, err)
		}
	}

	if err := safe.CleanupOutputs[*storage.LVMPhysicalVolumeStatus](ctx, r); err != nil {
		return fmt.Errorf("cleanup pv outputs: %w", err)
	}

	return nil
}

func (ctrl *LVMScanController) applyLVs(ctx context.Context, r controller.Runtime, lvs []lvm.LV) error {
	r.StartTrackingOutputs()

	for _, lv := range lvs {
		key := lv.FullName
		if key == "" {
			key = lv.Path
		}

		if err := safe.WriterModify(
			ctx, r,
			storage.NewLVMLogicalVolumeStatus(storage.NamespaceName, lvID(key)),
			func(s *storage.LVMLogicalVolumeStatus) error {
				spec := s.TypedSpec()
				spec.Path = lv.Path
				spec.DMPath = lv.DMPath
				spec.Name = lv.Name
				spec.FullName = lv.FullName
				spec.VGName = lv.VGName
				spec.UUID = lv.UUID
				spec.Layout = lv.Layout
				spec.Role = lv.Role
				spec.Permissions = lv.Permissions
				spec.AllocationPolicy = lv.AllocationPolicy
				spec.AllocationLocked = lv.AllocationLocked
				spec.FixedMinor = lv.FixedMinor
				spec.Active = lv.Active
				spec.ActiveLocally = lv.ActiveLocally
				spec.ActiveRemotely = lv.ActiveRemotely
				spec.ActiveExclusively = lv.ActiveExclusively
				spec.Suspended = lv.Suspended
				spec.DeviceOpen = lv.DeviceOpen
				spec.SkipActivation = lv.SkipActivation
				spec.Merging = lv.Merging
				spec.Converting = lv.Converting
				spec.Size = lv.Size
				spec.MetadataSize = lv.MetadataSize
				spec.ReadAhead = lv.ReadAhead
				spec.KernelMajor = lv.KernelMajor
				spec.KernelMinor = lv.KernelMinor
				spec.Origin = lv.Origin
				spec.OriginSize = lv.OriginSize
				spec.PoolLV = lv.PoolLV
				spec.DataLV = lv.DataLV
				spec.MetadataLV = lv.MetadataLV
				spec.MovePV = lv.MovePV
				spec.ConvertLV = lv.ConvertLV
				spec.WhenFull = lv.WhenFull
				spec.Tags = []string(lv.Tags)

				return nil
			},
		); err != nil {
			return fmt.Errorf("modify lv %q: %w", key, err)
		}
	}

	if err := safe.CleanupOutputs[*storage.LVMLogicalVolumeStatus](ctx, r); err != nil {
		return fmt.Errorf("cleanup lv outputs: %w", err)
	}

	return nil
}

// pvID derives a resource ID from a PV device path. /dev/sda1 → sda1.
// Slashes can't appear in COSI resource IDs.
func pvID(device string) string {
	return strings.TrimPrefix(strings.ReplaceAll(device, "/", "-"), "-dev-")
}

// lvID derives a resource ID from an LV path or full name.
// "/dev/vg0/data" → "vg0-data"; "vg0/data" → "vg0-data".
func lvID(key string) string {
	return strings.TrimPrefix(strings.ReplaceAll(key, "/", "-"), "-dev-")
}
