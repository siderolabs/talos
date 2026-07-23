// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"time"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// PromotableSystemVolumeNames are the system volumes that default to a directory under the
// EPHEMERAL volume but may instead be placed on a dedicated partition (via provisioning) at
// cluster creation. The backing (directory vs. dedicated partition) is fixed at creation time.
var PromotableSystemVolumeNames = []string{
	constants.EtcdDataVolumeID,
	constants.CRIContainerdVolumeID,
	constants.KubeletDataVolumeID,
	constants.LogVolumeID,
}

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
	Filesystem() SystemVolumeFilesystemConfig
	Encryption() EncryptionConfig
	Mount() VolumeMountConfig
	VolumeTrimConfigProvider
}

// VolumeProvisioningConfig defines the interface to access volume provisioning configuration.
type VolumeProvisioningConfig interface {
	DiskSelector() optional.Optional[cel.Expression]
	Grow() optional.Optional[bool]
	MinSize() optional.Optional[uint64]
	MaxSize() optional.Optional[uint64]
	RelativeMaxSize() optional.Optional[uint64]
	MaxSizeNegative() bool
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

	return emptyVolumeConfig{secure: name != constants.EphemeralPartitionLabel}, false
}

type emptyVolumeConfig struct {
	secure bool
}

func (emptyVolumeConfig) Name() string {
	return ""
}

func (emptyVolumeConfig) Provisioning() VolumeProvisioningConfig {
	return emptyVolumeConfig{}
}

func (config emptyVolumeConfig) Filesystem() SystemVolumeFilesystemConfig {
	return config
}

func (emptyVolumeConfig) XFS() XFSFilesystemConfig {
	return nil
}

func (emptyVolumeConfig) Encryption() EncryptionConfig {
	return nil
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

func (emptyVolumeConfig) RelativeMaxSize() optional.Optional[uint64] {
	return optional.None[uint64]()
}

func (emptyVolumeConfig) MaxSizeNegative() bool {
	return false
}

func (config emptyVolumeConfig) Mount() VolumeMountConfig {
	return emptyVolumeMountConfig(config)
}

func (emptyVolumeConfig) Trim() VolumeTrimConfig {
	return nil
}

type emptyVolumeMountConfig struct {
	secure bool
}

func (emptyVolumeMountConfig) DisableAccessTime() bool {
	return false
}

func (config emptyVolumeMountConfig) Secure() bool {
	return config.secure
}

func (emptyVolumeMountConfig) ReadOnly() bool {
	return false
}

// UserVolumeConfig defines the interface to access user volume configuration.
type UserVolumeConfig interface {
	NamedDocument
	UserVolumeConfigSignal()
	Type() optional.Optional[block.VolumeType]
	Provisioning() VolumeProvisioningConfig
	Filesystem() FilesystemConfig
	Encryption() EncryptionConfig
	Mount() UserVolumeMountConfig
	VolumeTrimConfigProvider
}

// RawVolumeConfig defines the interface to access raw volume configuration.
type RawVolumeConfig interface {
	NamedDocument
	RawVolumeConfigSignal()
	Provisioning() VolumeProvisioningConfig
	Encryption() EncryptionConfig
}

// ExistingVolumeConfig defines the interface to access existing volume configuration.
type ExistingVolumeConfig interface {
	NamedDocument
	ExistingVolumeConfigSignal()
	VolumeDiscovery() VolumeDiscoveryConfig
	Mount() ExistingVolumeMountConfig
	VolumeTrimConfigProvider
}

// ExternalVolumeConfig defines the interface to access external volume configuration.
type ExternalVolumeConfig interface {
	NamedDocument
	ExternalVolumeConfigSignal()
	Type() block.FilesystemType
	Mount() ExternalVolumeMountConfig
}

// VolumeDiscoveryConfig defines the interface to discover volumes.
//
//nolint:iface
type VolumeDiscoveryConfig interface {
	VolumeSelector() cel.Expression
}

// VolumeMountConfig defines the interface to access volume mount configuration.
type VolumeMountConfig interface {
	Secure() bool
	DisableAccessTime() bool
}

// UserVolumeMountConfig defines the interface to access volume mount configuration.
type UserVolumeMountConfig = VolumeMountConfig

// ExistingVolumeMountConfig defines the interface to access volume mount configuration.
type ExistingVolumeMountConfig interface {
	UserVolumeMountConfig
	ReadOnly() bool
}

// ExternalVolumeMountConfig defines the interface to access volume mount configuration.
type ExternalVolumeMountConfig interface {
	ExistingVolumeMountConfig
	Virtiofs() optional.Optional[ExternalVolumeMountConfigSpec]
}

// ExternalVolumeMountConfigSpec defines the interface to access external mount configuration spec.
type ExternalVolumeMountConfigSpec interface {
	Source() string
	Parameters() ([]block.ParameterSpec, error)
}

// FilesystemConfig defines the interface to access filesystem configuration.
type FilesystemConfig interface {
	SystemVolumeFilesystemConfig
	// Type returns the filesystem type.
	Type() block.FilesystemType
	// ProjectQuotaSupport returns true if the filesystem should support project quotas.
	ProjectQuotaSupport() bool
}

// SystemVolumeFilesystemConfig is the subset of the filesystem configuration which can be set for
// system volumes (the filesystem type is fixed, and project quota support comes from machine
// features).
type SystemVolumeFilesystemConfig interface {
	// XFS returns the XFS-specific filesystem configuration, if any.
	XFS() XFSFilesystemConfig
}

// XFSFilesystemConfig defines the interface to access XFS-specific filesystem configuration.
type XFSFilesystemConfig interface {
	// MinAllocationGroupSize returns the minimum XFS allocation group size in bytes.
	MinAllocationGroupSize() optional.Optional[uint64]
}

// SwapVolumeConfig defines the interface to access swap volume configuration.
type SwapVolumeConfig interface {
	NamedDocument
	SwapVolumeConfigSignal()
	Provisioning() VolumeProvisioningConfig
	Encryption() EncryptionConfig
}

// ZswapConfig defines the interface to access zswap configuration.
type ZswapConfig interface {
	ZswapConfigSignal()
	MaxPoolPercent() int
	ShrinkerEnabled() bool
}

// FilesystemTrimConfig defines the interface to access global filesystem trim configuration.
type FilesystemTrimConfig interface {
	FilesystemTrimConfigSignal()
	// Interval returns the global trim interval for filesystems which support trimming.
	Interval() time.Duration
}

// DiskSMARTConfig defines the interface to access disk SMART monitoring configuration.
type DiskSMARTConfig interface {
	DiskSMARTConfigSignal()
	// Enabled returns whether SMART status collection is enabled.
	Enabled() bool
	// Interval returns the interval at which disk SMART status is refreshed.
	Interval() time.Duration
}

// VolumeTrimConfigProvider defines the interface to access per-volume trim configuration.
type VolumeTrimConfigProvider interface {
	// Trim returns the per-volume trim configuration, or nil if not set.
	Trim() VolumeTrimConfig
}

// VolumeTrimConfig defines the interface to access per-volume filesystem trim configuration.
//
// It overrides the global filesystem trim configuration for the volume.
type VolumeTrimConfig interface {
	// Enabled returns whether trimming is enabled for the volume (if explicitly set).
	Enabled() optional.Optional[bool]
	// Interval returns the trim interval for the volume (if explicitly set), overriding the global interval.
	Interval() optional.Optional[time.Duration]
}
