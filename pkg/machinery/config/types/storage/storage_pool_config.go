// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

//docgen:jsonschema

import (
	"errors"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/storagehelpers"
)

// StoragePoolConfigKind is a config document kind.
const StoragePoolConfigKind = "StoragePoolConfig"

func init() {
	registry.Register(StoragePoolConfigKind, func(version string) config.Document {
		if version == "v1alpha1" { //nolint:goconst
			return &StoragePoolConfigV1Alpha1{}
		}

		return nil
	})
}

var (
	_ config.StoragePoolConfig = &StoragePoolConfigV1Alpha1{}
	_ config.Validator         = &StoragePoolConfigV1Alpha1{}
)

// StoragePoolConfigV1Alpha1 defines a directory storage pool on a configured filesystem volume.
//
//	description: |
//	  Defines a named storage pool backed by a UserVolumeConfig, ExistingVolumeConfig,
//	  or ExternalVolumeConfig. The backing volume must be writable and filesystem-backed.
//	  Only one pool may reference a backing volume. The pool directory is named after
//	  the pool beneath the volume mount target. Removing the configuration stops and
//	  undefines the pool, but never deletes its files or backing volume.
//	  Requires the libvirtd system extension. Changing the backing volume changes
//	  the pool target; it does not migrate existing disk images.
//	examples:
//	  - value: exampleStoragePoolConfigV1Alpha1()
//	alias: StoragePoolConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/StoragePoolConfig
type StoragePoolConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Pool name: 1-63 ASCII letters, digits, hyphens or underscores, starting with a letter or digit.
	MetaName string `yaml:"name"`
	//   description: |
	//     Reference to the writable filesystem volume backing this pool.
	VolumeConfig StoragePoolVolume `yaml:"volume"`
}

// StoragePoolVolume references a backing volume by its document name.
type StoragePoolVolume struct {
	//   description: |
	//     Name of the UserVolumeConfig, ExistingVolumeConfig, or ExternalVolumeConfig document.
	//     This is the literal document name, not the runtime volume ID.
	//     For example, a UserVolumeConfig named `u-images` is referenced as `u-images`.
	VolumeName string `yaml:"name"`
}

// NewStoragePoolConfigV1Alpha1 creates a StoragePoolConfig document.
func NewStoragePoolConfigV1Alpha1() *StoragePoolConfigV1Alpha1 {
	return &StoragePoolConfigV1Alpha1{
		Meta: meta.Meta{MetaKind: StoragePoolConfigKind, MetaAPIVersion: "v1alpha1"},
	}
}

func exampleStoragePoolConfigV1Alpha1() *StoragePoolConfigV1Alpha1 {
	cfg := NewStoragePoolConfigV1Alpha1()
	cfg.MetaName = "vm-images"
	cfg.VolumeConfig.VolumeName = "vm-data"

	return cfg
}

// Name implements config.NamedDocument.
func (s *StoragePoolConfigV1Alpha1) Name() string { return s.MetaName }

// Clone implements config.Document.
func (s *StoragePoolConfigV1Alpha1) Clone() config.Document { return s.DeepCopy() }

// StoragePoolConfigSignal implements config.StoragePoolConfig.
func (s *StoragePoolConfigV1Alpha1) StoragePoolConfigSignal() {}

// VolumeName implements config.StoragePoolConfig.
func (s *StoragePoolConfigV1Alpha1) VolumeName() string { return s.VolumeConfig.VolumeName }

// Validate implements config.Validator.
func (s *StoragePoolConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	if err := storagehelpers.ValidateStoragePoolName(s.MetaName); err != nil {
		errs = errors.Join(errs, err)
	}

	if s.VolumeConfig.VolumeName == "" {
		errs = errors.Join(errs, errors.New("volume.name is required"))
	}

	return nil, errs
}
