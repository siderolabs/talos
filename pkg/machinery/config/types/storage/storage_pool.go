// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

//docgen:jsonschema

import (
	"errors"
	"regexp"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// StoragePoolKind is a config document kind.
const StoragePoolKind = "StoragePool"

var storagePoolNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`)

func init() {
	registry.Register(StoragePoolKind, func(version string) config.Document {
		if version == "v1alpha1" { //nolint:goconst
			return &StoragePoolV1Alpha1{}
		}

		return nil
	})
}

var (
	_ config.StoragePoolConfig = &StoragePoolV1Alpha1{}
	_ config.Validator         = &StoragePoolV1Alpha1{}
)

// StoragePoolV1Alpha1 defines a directory storage pool on a configured filesystem volume.
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
//	  - value: exampleStoragePoolV1Alpha1()
//	alias: StoragePool
//	schemaRoot: true
//	schemaMeta: v1alpha1/StoragePool
type StoragePoolV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Pool name: 1-63 ASCII letters, digits, hyphens or underscores, starting with a letter or digit.
	MetaName string `yaml:"name"`
	//   description: |
	//     Reference to the writable filesystem volume backing this pool.
	VolumeConfig StoragePoolVolume `yaml:"volume"`
}

// StoragePoolVolume references a backing volume by its runtime volume ID.
type StoragePoolVolume struct {
	//   description: |
	//     Runtime volume ID: `u-<name>` for UserVolumeConfig, `e-<name>` for ExistingVolumeConfig,
	//     or `x-<name>` for ExternalVolumeConfig, where `<name>` is the document name.
	//     The prefix is required, even when the document name is unique.
	VolumeName string `yaml:"name"`
}

// NewStoragePoolV1Alpha1 creates a StoragePool document.
func NewStoragePoolV1Alpha1() *StoragePoolV1Alpha1 {
	return &StoragePoolV1Alpha1{
		Meta: meta.Meta{MetaKind: StoragePoolKind, MetaAPIVersion: "v1alpha1"},
	}
}

func exampleStoragePoolV1Alpha1() *StoragePoolV1Alpha1 {
	cfg := NewStoragePoolV1Alpha1()
	cfg.MetaName = "vm-images"
	cfg.VolumeConfig.VolumeName = "u-vm-data"

	return cfg
}

// Name implements config.NamedDocument.
func (s *StoragePoolV1Alpha1) Name() string { return s.MetaName }

// Clone implements config.Document.
func (s *StoragePoolV1Alpha1) Clone() config.Document { return s.DeepCopy() }

// StoragePoolConfigSignal implements config.StoragePoolConfig.
func (s *StoragePoolV1Alpha1) StoragePoolConfigSignal() {}

// VolumeName implements config.StoragePoolConfig.
func (s *StoragePoolV1Alpha1) VolumeName() string { return s.VolumeConfig.VolumeName }

// Validate implements config.Validator.
func (s *StoragePoolV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	if !storagePoolNamePattern.MatchString(s.MetaName) {
		errs = errors.Join(errs, errors.New("name must be 1-63 ASCII letters, digits, hyphens or underscores, starting with a letter or digit"))
	}

	if s.VolumeConfig.VolumeName == "" {
		errs = errors.Join(errs, errors.New("volume.name is required"))
	} else if name := s.VolumeConfig.VolumeName; len(name) <= len(constants.UserVolumePrefix) ||
		(!strings.HasPrefix(name, constants.UserVolumePrefix) &&
			!strings.HasPrefix(name, constants.ExistingVolumePrefix) &&
			!strings.HasPrefix(name, constants.ExternalVolumePrefix)) {
		errs = errors.Join(errs, errors.New("volume.name must be a runtime volume ID: u-<name>, e-<name>, or x-<name> with a non-empty name"))
	}

	return nil, errs
}
