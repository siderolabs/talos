// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package volumes provides utilities and extra functions for the volume manager.
package volumes

import (
	"cmp"

	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// CompareVolumeConfigs compares two volume configs in the proposed order of provisioning.
func CompareVolumeConfigs(a, b *block.VolumeConfig) int {
	if c := cmp.Compare(a.TypedSpec().Provisioning.Wave, b.TypedSpec().Provisioning.Wave); c != 0 {
		return c
	}

	return cmpBool(a.TypedSpec().Provisioning.PartitionSpec.Grow, b.TypedSpec().Provisioning.PartitionSpec.Grow)
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
	SystemDisk bool
}

// ManagerContext captures the context of the volume manager.
type ManagerContext struct {
	Cfg               *block.VolumeConfig
	Status            *block.VolumeStatusSpec
	DiscoveredVolumes []*blockpb.DiscoveredVolumeSpec
	Disks             []DiskContext

	DevicesReady            bool
	PreviousWaveProvisioned bool
	SystemInformation       *hardware.SystemInformation
	Lifecycle               *block.VolumeLifecycle
}
