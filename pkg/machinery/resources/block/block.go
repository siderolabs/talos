// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package block provides resources related to blockdevices, mounts, etc.
package block

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/maps"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

//go:generate deep-copy -type DeviceSpec -type DiscoveredVolumeSpec -type DiscoveryRefreshRequestSpec -type DiscoveryRefreshStatusSpec  -type DiskSpec -type MountRequestSpec -type MountStatusSpec -type SwapStatusSpec -type SymlinkSpec -type SystemDiskSpec -type UserDiskConfigStatusSpec -type VolumeConfigSpec -type VolumeLifecycleSpec -type VolumeMountRequestSpec -type VolumeMountStatusSpec -type VolumeStatusSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

//go:generate enumer -type=VolumeType,VolumePhase,FilesystemType,EncryptionKeyType,EncryptionProviderType  -linecomment -text

// NamespaceName contains configuration resources.
const NamespaceName resource.Namespace = v1alpha1.NamespaceName

// UserDiskLabel is the label for user disks.
const UserDiskLabel = "talos.dev/user-disk"

// UserVolumeLabel is the label for user volumes.
const UserVolumeLabel = "talos.dev/user-volume"

// SwapVolumeLabel is the label for swap volumes.
const SwapVolumeLabel = "talos.dev/swap-volume"

// PlatformLabel is the label for platform volumes.
const PlatformLabel = "talos.dev/platform"

// WaitForVolumePhase waits for the volume to reach the expected phase(s).
func WaitForVolumePhase(ctx context.Context, st state.State, volumeID string, expectedPhases ...VolumePhase) (*VolumeStatus, error) {
	volumeStatus, err := st.WatchFor(ctx,
		NewVolumeStatus(NamespaceName, volumeID).Metadata(),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			volumeStatus, ok := r.(*VolumeStatus)
			if !ok {
				return false, nil
			}

			return slices.Index(expectedPhases, volumeStatus.TypedSpec().Phase) != -1, nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("error waiting for volume %q to be ready: %w", volumeID, err)
	}

	return volumeStatus.(*VolumeStatus), nil
}

// GetSystemDisk returns the system disk from the state.
//
// If the system disk is not found, it returns nil.
func GetSystemDisk(ctx context.Context, st state.State) (*SystemDiskSpec, error) {
	systemDisk, err := safe.StateGetByID[*SystemDisk](ctx, st, SystemDiskID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error getting system disk: %w", err)
	}

	return systemDisk.TypedSpec(), nil
}

// GetSystemDiskPaths returns the path(s) of system disk and STATE/EPHEMERAL partitions.
//
// This is a legacy method to map old concept of system disk wipe into new volume subsystem.
func GetSystemDiskPaths(ctx context.Context, st state.State) ([]string, error) {
	systemDisks := map[string]struct{}{}

	// fetch system disk (where Talos is installed)
	systemDisk, err := GetSystemDisk(ctx, st)
	if err != nil {
		return nil, err
	}

	if systemDisk != nil {
		systemDisks[systemDisk.DevPath] = struct{}{}
	}

	// fetch additional system volumes (which might be on the same or other disks)
	for _, volumeID := range []string{constants.StatePartitionLabel, constants.EphemeralPartitionLabel} {
		volumeStatus, err := safe.ReaderGetByID[*VolumeStatus](ctx, st, volumeID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return nil, err
		}

		if volumeStatus.TypedSpec().ParentLocation != "" {
			systemDisks[volumeStatus.TypedSpec().ParentLocation] = struct{}{}
		} else if volumeStatus.TypedSpec().Location != "" {
			systemDisks[volumeStatus.TypedSpec().Location] = struct{}{}
		}
	}

	return maps.Keys(systemDisks), nil
}
