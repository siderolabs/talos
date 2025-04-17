// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package volumes provides utilities and extra functions for the volume manager.
package volumes

import (
	"cmp"
	"context"
	"math"

	"github.com/siderolabs/gen/optional"

	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// CompareVolumeConfigs compares two volume configs in the proposed order of provisioning.
func CompareVolumeConfigs(a, b *block.VolumeConfig) int {
	// first, sort by wave, smaller wave first
	if c := cmp.Compare(a.TypedSpec().Provisioning.Wave, b.TypedSpec().Provisioning.Wave); c != 0 {
		return c
	}

	// prefer partitions which do not grow, as growing partitions may consume space needed by other partitions
	if c := cmpBool(a.TypedSpec().Provisioning.PartitionSpec.Grow, b.TypedSpec().Provisioning.PartitionSpec.Grow); c != 0 {
		return c
	}

	// prefer partitions with smaller sizes first
	// e.g.: for a disk of size 1GiB, and following config with min-max requested sizes:
	// 1. 100MiB - 200MiB
	// 2. 300MiB - 2GiB
	//
	// if the order is 2-1, the second partition will grow to full disk size and will leave no space for the first partition,
	// but if the order is 1-2, partition sizes will 200MiB and 800MiB respectively.
	//
	// we compare only max size, as it affects the resulting size of the partition
	desiredSizeA := cmp.Or(a.TypedSpec().Provisioning.PartitionSpec.MaxSize, math.MaxUint64)
	desiredSizeB := cmp.Or(b.TypedSpec().Provisioning.PartitionSpec.MaxSize, math.MaxUint64)

	return cmp.Compare(desiredSizeA, desiredSizeB)
}

func cmpBool(a, b bool) int {
	if a == b {
		return 0
	}

	if a {
		return 1
	}

	return -1
}

// Retryable is an error tag.
type Retryable struct{}

// DiskContext captures the context of a disk.
type DiskContext struct {
	Disk       *blockpb.DiskSpec
	SystemDisk optional.Optional[bool]
}

// ToCELContext converts the disk context to CEL contexts.
func (d *DiskContext) ToCELContext() map[string]any {
	result := map[string]any{
		"disk": d.Disk,
	}

	if val, ok := d.SystemDisk.Get(); ok {
		result["system_disk"] = val
	}

	return result
}

// ManagerContext captures the context of the volume manager.
type ManagerContext struct {
	Cfg               *block.VolumeConfig
	Status            *block.VolumeStatusSpec
	ParentStatus      *block.VolumeStatus
	ParentFinalizer   string
	DiscoveredVolumes []*blockpb.DiscoveredVolumeSpec
	Disks             []DiskContext

	DevicesReady            bool
	PreviousWaveProvisioned bool
	GetSystemInformation    func(context.Context) (*hardware.SystemInformation, error)
	TPMLocker               func(context.Context, func() error) error
	ShouldCloseVolume       bool
}
