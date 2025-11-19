// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"strings"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// ExternalVolumeConfig is a config document kind.
const ExternalVolumeConfigKind = "ExternalVolumeConfig"

func init() {
	registry.Register(ExternalVolumeConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &ExternalVolumeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.ExternalVolumeConfig = &ExternalVolumeConfigV1Alpha1{}
	_ config.NamedDocument        = &ExternalVolumeConfigV1Alpha1{}
	_ config.Validator            = &ExternalVolumeConfigV1Alpha1{}
)

const maxExternalVolumeNameLength = constants.PartitionLabelLength - len(constants.ExternalVolumePrefix)

// FilesystemType is an alias for block.FilesystemType.
type FilesystemType = block.FilesystemType

// NFSVersionType is an alias for block.NFSVersionType.
type NFSVersionType = block.NFSVersionType

// ExternalVolumeConfigV1Alpha1 is a external disk mount configuration document.
//
//	description: |
//	  External volumes allow to mount volumes that were created outside of Talos,
//	  over the network or API. Volume will be mounted under `/var/mnt/<name>`.
//	  The external volume config name should not conflict with user volume names.
//	examples:
//	  - value: exampleExternalVolumeConfigV1Alpha1Virtiofs()
//	  - value: exampleExternalVolumeConfigV1Alpha1NFS()
//	alias: ExternalVolumeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/ExternalVolumeConfig
type ExternalVolumeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the mount.
	//
	//     Name might be between 1 and 34 characters long and can only contain:
	//     lowercase and uppercase ASCII letters, digits, and hyphens.
	MetaName string `yaml:"name"`
	//   description: |
	//     Filesystem type.
	//   values:
	//     - virtiofs
	//     - nfs
	//  schema:
	//    type: string
	FilesystemType FilesystemType `yaml:"filesystemType"`
	//   description: |
	//     The mount describes additional mount options.
	MountSpec ExternalMountSpec `yaml:"mount,omitempty"`
}

// ExternalMountSpec describes how the external volume is mounted.
type ExternalMountSpec struct {
	//   description: |
	//     Mount the volume read-only.
	MountReadOnly *bool `yaml:"readOnly,omitempty"`

	//   description: |
	//     Source of the volume.
	MountSource string `yaml:"source"`

	//   description: |
	//     NFS mount options.
	MountNFS *NFSMountSpec `yaml:"nfs,omitempty"`
}

// NOTE: to not forget mappings https://man7.org/linux/man-pages/man5/nfs.5.html

// NFSMountSpec describes NFS mount options.
type NFSMountSpec struct {
	//   description: |
	//     NFS version to use.
	//   values:
	//     - 4.2
	//     - 4.1
	//     - 4
	//     - 3
	//     - 2
	//  schema:
	//    type: string
	NFSVersion NFSVersionType `yaml:"version"`
}

// NewExternalVolumeConfigV1Alpha1 creates a new user mount config document.
func NewExternalVolumeConfigV1Alpha1() *ExternalVolumeConfigV1Alpha1 {
	return &ExternalVolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       ExternalVolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleExternalVolumeConfigV1Alpha1Virtiofs() *ExternalVolumeConfigV1Alpha1 {
	cfg := NewExternalVolumeConfigV1Alpha1()
	cfg.MetaName = "mount1"
	cfg.FilesystemType = block.FilesystemTypeVirtiofs
	cfg.MountSpec.MountSource = "Data"

	return cfg
}

func exampleExternalVolumeConfigV1Alpha1NFS() *ExternalVolumeConfigV1Alpha1 {
	cfg := NewExternalVolumeConfigV1Alpha1()
	cfg.MetaName = "mount1"
	cfg.FilesystemType = block.FilesystemTypeVirtiofs
	cfg.MountSpec.MountSource = "10.2.21.1:/backups"
	cfg.MountSpec.MountNFS = &NFSMountSpec{
		NFSVersion: block.NFSVersionType4_2,
	}

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *ExternalVolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *ExternalVolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo,dupl
func (s *ExternalVolumeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		warnings         []string
		validationErrors error
	)

	if s.MetaName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	}

	if len(s.MetaName) < 1 || len(s.MetaName) > maxExternalVolumeNameLength {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("name must be between 1 and %d characters long", maxExternalVolumeNameLength))
	}

	if strings.ContainsFunc(s.MetaName, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= 'A' && r <= 'Z':
			return false
		case r >= '0' && r <= '9':
			return false
		case r == '-':
			return false
		default: // invalid symbol
			return true
		}
	}) {
		validationErrors = errors.Join(validationErrors, errors.New("name can only contain lowercase and uppercase ASCII letters, digits, and hyphens"))
	}

	if s.MountSpec.MountSource == "" {
		validationErrors = errors.Join(validationErrors, errors.New("mount source is required"))
	}

	switch s.FilesystemType {
	case block.FilesystemTypeNFS:
		extraWarnings, extraErrors := s.MountSpec.MountNFS.Validate()

		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

	case block.FilesystemTypeVirtiofs:
		// TODO: virtiofs validation

	case block.FilesystemTypeNone, block.FilesystemTypeXFS, block.FilesystemTypeVFAT, block.FilesystemTypeEXT4, block.FilesystemTypeISO9660, block.FilesystemTypeSwap:
		fallthrough

	default:
		validationErrors = errors.Join(validationErrors, fmt.Errorf("invalid filesystem type: %s", s.FilesystemType))
	}

	return warnings, validationErrors
}

// ExternalVolumeConfigSignal is a signal for user mount config.
func (s *ExternalVolumeConfigV1Alpha1) ExternalVolumeConfigSignal() {}

// Type implements config.ExternalVolumeConfig interface.
func (s *ExternalVolumeConfigV1Alpha1) Type() FilesystemType {
	return s.FilesystemType
}

// Mount implements config.ExternalVolumeConfig interface.
func (s *ExternalVolumeConfigV1Alpha1) Mount() config.ExternalMountConfig {
	return s.MountSpec
}

// ReadOnly implements config.VolumeMountConfig interface.
func (s ExternalMountSpec) ReadOnly() bool {
	return pointer.SafeDeref(s.MountReadOnly)
}

// Source implements config.VolumeMountConfig interface.
func (s ExternalMountSpec) Source() string {
	return s.MountSource
}

// NFS implements config.VolumeMountConfig interface.
func (s ExternalMountSpec) NFS() optional.Optional[config.NFSMountConfig] {
	if s.MountNFS == nil {
		return optional.None[config.NFSMountConfig]()
	}

	return optional.Some[config.NFSMountConfig](*s.MountNFS)
}

// Version implements config.NFSMountConfig interface.
func (s NFSMountSpec) Version() string {
	return s.NFSVersion.String()
}

// Validate implements config.Validator interface.
func (s *NFSMountSpec) Validate() ([]string, error) {
	var validationErrors error

	if s == nil {
		return nil, validationErrors
	}

	return nil, validationErrors
}
