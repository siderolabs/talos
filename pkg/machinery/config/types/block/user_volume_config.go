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

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// UserVolumeConfigKind is a config document kind.
const UserVolumeConfigKind = "UserVolumeConfig"

func init() {
	registry.Register(UserVolumeConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &UserVolumeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.UserVolumeConfig    = &UserVolumeConfigV1Alpha1{}
	_ config.ConflictingDocument = &UserVolumeConfigV1Alpha1{}
	_ config.NamedDocument       = &UserVolumeConfigV1Alpha1{}
	_ config.Validator           = &UserVolumeConfigV1Alpha1{}
)

const maxUserVolumeNameLength = constants.PartitionLabelLength - len(constants.UserVolumePrefix)

// VolumeType is an alias for block.VolumeType.
type VolumeType = block.VolumeType

// UserVolumeConfigV1Alpha1 is a user volume configuration document.
//
//	description: |
//	  User volume is automatically allocated as a partition on the specified disk
//	  and mounted under `/var/mnt/<name>`.
//	  The partition label is automatically generated as `u-<name>`.
//	examples:
//	  - value: exampleUserVolumeConfigV1Alpha1Directory()
//	  - value: exampleUserVolumeConfigV1Alpha1Partition()
//	alias: UserVolumeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/UserVolumeConfig
type UserVolumeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the volume.
	//
	//     Name might be between 1 and 34 characters long and can only contain:
	//     lowercase and uppercase ASCII letters, digits, and hyphens.
	MetaName string `yaml:"name"`
	//   description: |
	//     Volume type.
	//   values:
	//     - partition
	//     - directory
	//  schema:
	//    type: string
	VolumeType *VolumeType `yaml:"volumeType,omitempty"`
	//   description: |
	//     The provisioning describes how the volume is provisioned.
	ProvisioningSpec ProvisioningSpec `yaml:"provisioning,omitempty"`
	//   description: |
	//     The filesystem describes how the volume is formatted.
	FilesystemSpec FilesystemSpec `yaml:"filesystem,omitempty"`
	//   description: |
	//     The encryption describes how the volume is encrypted.
	EncryptionSpec EncryptionSpec `yaml:"encryption,omitempty"`
}

// NewUserVolumeConfigV1Alpha1 creates a new user volume config document.
func NewUserVolumeConfigV1Alpha1() *UserVolumeConfigV1Alpha1 {
	return &UserVolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       UserVolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

const userVolumeName = "local-data"

func exampleUserVolumeConfigV1Alpha1Partition() *UserVolumeConfigV1Alpha1 {
	cfg := NewUserVolumeConfigV1Alpha1()
	cfg.MetaName = userVolumeName
	cfg.VolumeType = pointer.To(block.VolumeTypePartition)
	cfg.ProvisioningSpec = ProvisioningSpec{
		DiskSelectorSpec: DiskSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "nvme"`, celenv.DiskLocator())),
		},
		ProvisioningMaxSize: MustByteSize("50GiB"),
	}
	cfg.FilesystemSpec = FilesystemSpec{
		FilesystemType: block.FilesystemTypeXFS,
	}
	cfg.EncryptionSpec = EncryptionSpec{
		EncryptionProvider: block.EncryptionProviderLUKS2,
		EncryptionKeys: []EncryptionKey{
			{
				KeySlot: 0,
				KeyTPM:  &EncryptionKeyTPM{},
			},
			{
				KeySlot: 1,
				KeyStatic: &EncryptionKeyStatic{
					KeyData: "topsecret",
				},
			},
		},
	}

	return cfg
}

func exampleUserVolumeConfigV1Alpha1Directory() *UserVolumeConfigV1Alpha1 {
	cfg := NewUserVolumeConfigV1Alpha1()
	cfg.MetaName = userVolumeName
	cfg.VolumeType = pointer.To(block.VolumeTypeDirectory)

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *UserVolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *UserVolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *UserVolumeConfigV1Alpha1) ConflictsWithKinds() []string {
	return []string{ExistingVolumeConfigKind}
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo,cyclop
func (s *UserVolumeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		warnings         []string
		validationErrors error
	)

	if s.MetaName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	}

	if len(s.MetaName) < 1 || len(s.MetaName) > maxUserVolumeNameLength {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("name must be between 1 and %d characters long", maxUserVolumeNameLength))
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

	vtype := block.VolumeTypePartition
	if s.VolumeType != nil {
		vtype = *s.VolumeType
	}

	switch vtype {
	case block.VolumeTypePartition:
		extraWarnings, extraErrors := s.ProvisioningSpec.Validate(true)

		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

		extraWarnings, extraErrors = s.FilesystemSpec.Validate()
		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

		extraWarnings, extraErrors = s.EncryptionSpec.Validate()
		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

	case block.VolumeTypeDirectory:
		if !s.ProvisioningSpec.IsZero() {
			validationErrors = errors.Join(validationErrors, errors.New("provisioning spec is invalid for volumeType directory"))
		}

		if !s.EncryptionSpec.IsZero() {
			validationErrors = errors.Join(validationErrors, errors.New("encryption spec is invalid for volumeType directory"))
		}

		if !s.FilesystemSpec.IsZero() {
			validationErrors = errors.Join(validationErrors, errors.New("filesystem spec is invalid for volumeType directory"))
		}

	case block.VolumeTypeDisk, block.VolumeTypeTmpfs, block.VolumeTypeSymlink, block.VolumeTypeOverlay:
		fallthrough

	default:
		validationErrors = errors.Join(validationErrors, fmt.Errorf("unsupported volume type %q", vtype))
	}

	return warnings, validationErrors
}

// UserVolumeConfigSignal is a signal for user volume config.
func (s *UserVolumeConfigV1Alpha1) UserVolumeConfigSignal() {}

// Type implements config.UserVolumeConfig interface.
func (s *UserVolumeConfigV1Alpha1) Type() optional.Optional[VolumeType] {
	if s.VolumeType == nil {
		return optional.None[VolumeType]()
	}

	return optional.Some(*s.VolumeType)
}

// Provisioning implements config.UserVolumeConfig interface.
func (s *UserVolumeConfigV1Alpha1) Provisioning() config.VolumeProvisioningConfig {
	return s.ProvisioningSpec
}

// Filesystem implements config.UserVolumeConfig interface.
func (s *UserVolumeConfigV1Alpha1) Filesystem() config.FilesystemConfig {
	return s.FilesystemSpec
}

// Encryption implements config.UserVolumeConfig interface.
func (s *UserVolumeConfigV1Alpha1) Encryption() config.EncryptionConfig {
	if s.EncryptionSpec.EncryptionProvider == block.EncryptionProviderNone {
		return nil
	}

	return s.EncryptionSpec
}

// FilesystemSpec configures the filesystem for the volume.
type FilesystemSpec struct {
	//   description: |
	//     Filesystem type. Default is `xfs`.
	//   values:
	//     - ext4
	//     - xfs
	FilesystemType block.FilesystemType `yaml:"type,omitempty"`
	//   description: |
	//     Enables project quota support, valid only for 'xfs' filesystem.
	//
	//     Note: changing this value might require a full remount of the filesystem.
	ProjectQuotaSupportConfig *bool `yaml:"projectQuotaSupport,omitempty"`
}

// IsZero checks if the filesystem spec is zero.
func (s FilesystemSpec) IsZero() bool {
	return s.FilesystemType == block.FilesystemTypeNone && s.ProjectQuotaSupportConfig == nil
}

// Type implements config.FilesystemConfig interface.
func (s FilesystemSpec) Type() block.FilesystemType {
	if s.FilesystemType == block.FilesystemTypeNone {
		return block.FilesystemTypeXFS
	}

	return s.FilesystemType
}

// ProjectQuotaSupport implements config.FilesysteemConfig interface.
func (s FilesystemSpec) ProjectQuotaSupport() bool {
	return pointer.SafeDeref(s.ProjectQuotaSupportConfig)
}

// Validate implements config.Validator interface.
func (s FilesystemSpec) Validate() ([]string, error) {
	switch s.FilesystemType { //nolint:exhaustive
	case block.FilesystemTypeNone:
	case block.FilesystemTypeXFS:
	case block.FilesystemTypeEXT4:
	default:
		return nil, fmt.Errorf("unsupported filesystem type: %s", s.FilesystemType)
	}

	if pointer.SafeDeref(s.ProjectQuotaSupportConfig) && s.Type() != block.FilesystemTypeXFS {
		return nil, fmt.Errorf("project quota support is only available for xfs filesystem")
	}

	return nil, nil
}
