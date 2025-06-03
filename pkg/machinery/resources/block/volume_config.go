// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"os"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/yamlutils"
)

// VolumeConfigType is type of VolumeConfig resource.
const VolumeConfigType = resource.Type("VolumeConfigs.block.talos.dev")

// VolumeConfig resource contains configuration for machine volumes.
type VolumeConfig = typed.Resource[VolumeConfigSpec, VolumeConfigExtension]

// VolumeConfigSpec is the spec for VolumeConfig resource.
//
//gotagsrewrite:gen
type VolumeConfigSpec struct {
	// Parent volume ID, if set no operations on the volume continue until the parent volume is ready.
	ParentID string `yaml:"parentId,omitempty" protobuf:"1"`

	// Volume type.
	Type VolumeType `yaml:"type" protobuf:"2"`

	// Provisioning configuration (how to provision a volume).
	Provisioning ProvisioningSpec `yaml:"provisioning" protobuf:"3"`

	// Encryption configuration (how to encrypt a volume).
	Encryption EncryptionSpec `yaml:"encryption,omitempty" protobuf:"6"`

	// How to find a volume.
	Locator LocatorSpec `yaml:"locator" protobuf:"4"`

	// Mount options for the volume.
	Mount MountSpec `yaml:"mount,omitempty" protobuf:"5"`

	// Symlink options for the volume.
	Symlink SymlinkProvisioningSpec `yaml:"symlink,omitempty" protobuf:"7"`
}

// Wave constants.
const (
	WaveSystemDisk      = -1
	WaveUserVolumes     = 0
	WaveLegacyUserDisks = 1000000 // legacy user disks rely on specific order of provisioning
)

// ProvisioningSpec is the spec for volume provisioning.
//
//gotagsrewrite:gen
type ProvisioningSpec struct {
	// Provisioning wave for the volume.
	//
	// Waves are processed sequentially - the volumes in the wave are only provisioned after the previous wave is done.
	Wave int `yaml:"wave,omitempty" protobuf:"3"`

	// DiskSelector selects a disk for the volume.
	DiskSelector DiskSelector `yaml:"diskSelector,omitempty" protobuf:"1"`

	// PartitionSpec describes how to provision the volume (partition type).
	PartitionSpec PartitionSpec `yaml:"partitionSpec,omitempty" protobuf:"2"`

	// FilesystemSpec describes how to provision the volume (filesystem type).
	FilesystemSpec FilesystemSpec `yaml:"filesystemSpec,omitempty" protobuf:"4"`
}

// DiskSelector selects a disk for the volume.
//
//gotagsrewrite:gen
type DiskSelector struct {
	Match cel.Expression `yaml:"match,omitempty" protobuf:"1"`
}

// PartitionSpec is the spec for volume partitioning.
//
//gotagsrewrite:gen
type PartitionSpec struct {
	// Partition minimum size in bytes.
	MinSize uint64 `yaml:"minSize" protobuf:"1"`

	// Partition maximum size in bytes, if not set, grows to the maximum size.
	MaxSize uint64 `yaml:"maxSize,omitempty" protobuf:"2"`

	// Grow the partition automatically to the maximum size.
	Grow bool `yaml:"grow" protobuf:"3"`

	// Label for the partition.
	Label string `yaml:"label,omitempty" protobuf:"4"`

	// Partition type UUID.
	TypeUUID string `yaml:"typeUUID,omitempty" protobuf:"5"`
}

// LocatorSpec is the spec for volume locator.
//
//gotagsrewrite:gen
type LocatorSpec struct {
	Match cel.Expression `yaml:"match,omitempty" protobuf:"1"`
}

// FilesystemSpec is the spec for volume filesystem.
//
//gotagsrewrite:gen
type FilesystemSpec struct {
	// Filesystem type.
	Type FilesystemType `yaml:"type" protobuf:"1"`
	// Filesystem label.
	Label string `yaml:"label,omitempty" protobuf:"2"`
}

// EncryptionSpec is the spec for volume encryption.
//
//gotagsrewrite:gen
type EncryptionSpec struct {
	Provider    EncryptionProviderType `yaml:"provider" protobuf:"1"`
	Keys        []EncryptionKey        `yaml:"keys" protobuf:"2"`
	Cipher      string                 `yaml:"cipher,omitempty" protobuf:"3"`
	KeySize     uint                   `yaml:"keySize,omitempty" protobuf:"4"`
	BlockSize   uint64                 `yaml:"blockSize,omitempty" protobuf:"5"`
	PerfOptions []string               `yaml:"perfOptions,omitempty" protobuf:"6"`
}

// EncryptionKey is the spec for volume encryption key.
//
//gotagsrewrite:gen
type EncryptionKey struct {
	Slot int               `yaml:"slot" protobuf:"1"`
	Type EncryptionKeyType `yaml:"type" protobuf:"2"`

	// Only for Type == "static":
	StaticPassphrase yamlutils.StringBytes `yaml:"staticPassphrase,omitempty" protobuf:"3"`

	// Only for Type == "kms":
	KMSEndpoint string `yaml:"kmsEndpoint,omitempty" protobuf:"4"`

	// Only for Type == "tpm":
	TPMCheckSecurebootStatusOnEnroll bool `yaml:"tpmCheckSecurebootStatusOnEnroll,omitempty" protobuf:"5"`
}

// MountSpec is the spec for volume mount.
//
//gotagsrewrite:gen
type MountSpec struct {
	// Mount path for the volume.
	TargetPath string `yaml:"targetPath" protobuf:"1"`
	// SELinux label for the volume.
	SelinuxLabel string `yaml:"selinuxLabel" protobuf:"2"`
	// Enable project quota (xfs) for the volume.
	ProjectQuotaSupport bool `yaml:"projectQuotaSupport" protobuf:"3"`
	// Parent mount request ID.
	ParentID string `yaml:"parentId,omitempty" protobuf:"4"`
	// FileMode is the file mode for the mount target.
	FileMode os.FileMode `yaml:"fileMode,omitempty" protobuf:"5"`
	// UID is the user ID for the mount target.
	UID int `yaml:"uid,omitempty" protobuf:"6"`
	// GID is the group ID for the mount target.
	GID int `yaml:"gid,omitempty" protobuf:"7"`
	// RecursiveRelabel is the recursive relabel/chown flag for the mount target.
	RecursiveRelabel bool `yaml:"recursiveRelabel,omitempty" protobuf:"8"`
}

// SymlinkProvisioningSpec is the spec for volume symlink.
//
//gotagsrewrite:gen
type SymlinkProvisioningSpec struct {
	// Symlink target path for the volume.
	SymlinkTargetPath string `yaml:"symlinkTargetPath" protobuf:"1"`
	// Force symlink creation.
	Force bool `yaml:"force" protobuf:"2"`
}

// NewVolumeConfig initializes a BlockVolumeConfig resource.
func NewVolumeConfig(namespace resource.Namespace, id resource.ID) *VolumeConfig {
	return typed.NewResource[VolumeConfigSpec, VolumeConfigExtension](
		resource.NewMetadata(namespace, VolumeConfigType, id, resource.VersionUndefined),
		VolumeConfigSpec{},
	)
}

// VolumeConfigExtension is auxiliary resource data for BlockVolumeConfig.
type VolumeConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VolumeConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VolumeConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[VolumeConfigSpec](VolumeConfigType, &VolumeConfig{})
	if err != nil {
		panic(err)
	}
}
