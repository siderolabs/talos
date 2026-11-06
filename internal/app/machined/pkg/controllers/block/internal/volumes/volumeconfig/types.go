// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig

import (
	machinedruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// VolumeResource is an internal type containing the information required by
// VolumeConfigController to create a VolumeConfig (and optionally
// VolumeMountRequest) resource.
//
// Transformers (System and User) transform the config into VolumeResources, which are
// later created by VolumeConfigController.
type VolumeResource struct {
	VolumeID           string                                  // ID of the volume to create.
	Label              string                                  // label of the volume to create.
	TransformFunc      func(vc *block.VolumeConfig) error      // func that applies the changes to the provided VolumeConfig.
	MountTransformFunc func(m *block.VolumeMountRequest) error // func that applies the changes to the provided VolumeMountRequest.
}

type volumeConfigTransformer func(c configconfig.Config) ([]VolumeResource, error)

// SkipUserVolumeMountRequest is used to skip creating a VolumeMountRequest for a user volume.
type SkipUserVolumeMountRequest struct{}

// minAllocationGroupSize resolves the effective XFS minimum allocation group size: the configured
// override if there is one, and the Talos default otherwise.
func minAllocationGroupSize(filesystem configconfig.SystemVolumeFilesystemConfig) uint64 {
	if filesystem != nil {
		if xfs := filesystem.XFS(); xfs != nil {
			if size, ok := xfs.MinAllocationGroupSize().Get(); ok {
				return size
			}
		}
	}

	return quirks.New("").XFSMinAllocationGroupSize()
}

// MetaProvider wraps acquiring meta.
type MetaProvider interface {
	Meta() machinedruntime.Meta
}
