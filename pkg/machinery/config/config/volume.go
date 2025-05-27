// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// VolumesConfig defines the interface to access volume configuration.
type VolumesConfig interface {
	// ByName returns a volume config configuration by name.
	//
	// If the configuration is missing, the method a stub which returns implements 'nothing is set' stub.
	ByName(name string) (VolumeConfig, bool)
}

// VolumeConfig defines the interface to access volume configuration.
type VolumeConfig interface {
	NamedDocument
	Provisioning() VolumeProvisioningConfig
}

// VolumeProvisioningConfig defines the interface to access volume provisioning configuration.
type VolumeProvisioningConfig interface {
	DiskSelector() optional.Optional[cel.Expression]
	Grow() optional.Optional[bool]
	MinSize() optional.Optional[uint64]
	MaxSize() optional.Optional[uint64]
}

// WrapVolumesConfigList wraps a list of VolumeConfig providing access by name.
func WrapVolumesConfigList(configs ...VolumeConfig) VolumesConfig {
	return volumesConfigWrapper(configs)
}

type volumesConfigWrapper []VolumeConfig

func (w volumesConfigWrapper) ByName(name string) (VolumeConfig, bool) {
	for _, doc := range w {
		if doc.Name() == name {
			return doc, true
		}
	}

	return emptyVolumeConfig{}, false
}

type emptyVolumeConfig struct{}

func (emptyVolumeConfig) Name() string {
	return ""
}

func (emptyVolumeConfig) Provisioning() VolumeProvisioningConfig {
	return emptyVolumeConfig{}
}

func (emptyVolumeConfig) DiskSelector() optional.Optional[cel.Expression] {
	return optional.None[cel.Expression]()
}

func (emptyVolumeConfig) Grow() optional.Optional[bool] {
	return optional.None[bool]()
}

func (emptyVolumeConfig) MinSize() optional.Optional[uint64] {
	return optional.None[uint64]()
}

func (emptyVolumeConfig) MaxSize() optional.Optional[uint64] {
	return optional.None[uint64]()
}

// UserVolumeConfig defines the interface to access user volume configuration.
type UserVolumeConfig interface {
	NamedDocument
	UserVolumeConfigSignal()
	Provisioning() VolumeProvisioningConfig
	Filesystem() FilesystemConfig
	Encryption() EncryptionConfig
}

// FilesystemConfig defines the interface to access filesystem configuration.
type FilesystemConfig interface {
	// Type returns the filesystem type.
	Type() block.FilesystemType
}

// SwapVolumeConfig defines the interface to access swap volume configuration.
type SwapVolumeConfig interface {
	NamedDocument
	SwapVolumeConfigSignal()
	Provisioning() VolumeProvisioningConfig
	Encryption() EncryptionConfig
}
