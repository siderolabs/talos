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

// ExternalVolumeConfigKind is a config document kind.
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

// ExternalVolumeConfigV1Alpha1 is an external disk mount configuration document.
//
//	description: |
//	  External volumes allow to mount volumes that were created outside of Talos,
//	  over the network or API. Volume will be mounted under `/var/mnt/<name>`.
//	  The external volume config name should not conflict with user volume names.
//	examples:
//	  - value: exampleExternalVolumeConfigV1Alpha1Virtiofs()
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
	//     Virtiofs mount options.
	MountVirtiofs *VirtiofsMountSpec `yaml:"virtiofs,omitempty"`
}

// VirtiofsMountSpec describes Virtiofs mount options.
type VirtiofsMountSpec struct {
	//   description: |
	//     Selector tag for the Virtiofs mount.
	VirtiofsTag string `yaml:"tag"`
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
	cfg.MountSpec.MountVirtiofs = &VirtiofsMountSpec{
		VirtiofsTag: "Data",
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

	switch s.FilesystemType {
	case block.FilesystemTypeVirtiofs:
		extraWarnings, extraErrors := s.MountSpec.MountVirtiofs.Validate()

		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

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

// Virtiofs implements config.VolumeMountConfig interface.
func (s ExternalMountSpec) Virtiofs() optional.Optional[config.ExternalMountConfigSpec] {
	if s.MountVirtiofs == nil {
		return optional.None[config.ExternalMountConfigSpec]()
	}

	return optional.Some[config.ExternalMountConfigSpec](*s.MountVirtiofs)
}

// Source implements config.ExternalMountConfigSpec interface.
func (s VirtiofsMountSpec) Source() string {
	return s.VirtiofsTag
}

// Parameters implements config.NFSMountConfig interface.
func (s VirtiofsMountSpec) Parameters() ([]block.ParameterSpec, error) {
	return nil, nil
}

// Validate implements config.Validator interface.
func (s *VirtiofsMountSpec) Validate() ([]string, error) {
	var validationErrors error

	if s == nil {
		return nil, errors.New("virtiofs mount spec is required")
	}

	if s.VirtiofsTag == "" {
		validationErrors = errors.Join(validationErrors, errors.New("virtiofs tag is required"))
	}

	return nil, validationErrors
}
