// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"strings"

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
	_ config.UserVolumeConfig = &UserVolumeConfigV1Alpha1{}
	_ config.NamedDocument    = &UserVolumeConfigV1Alpha1{}
	_ config.Validator        = &UserVolumeConfigV1Alpha1{}
)

const maxUserVolumeNameLength = constants.PartitionLabelLength - len(constants.UserVolumePrefix)

// UserVolumeConfigV1Alpha1 is a user volume configuration document.
//
//	description: |
//	  User volume is automatically allocated as a partition on the specified disk
//	  and mounted under `/var/mnt/<name>`.
//	  The partition label is automatically generated as `u-<name>`.
//	examples:
//	  - value: exampleUserVolumeConfigV1Alpha1()
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

func exampleUserVolumeConfigV1Alpha1() *UserVolumeConfigV1Alpha1 {
	cfg := NewUserVolumeConfigV1Alpha1()
	cfg.MetaName = "ceph-data"
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

// Name implements config.NamedDocument interface.
func (s *UserVolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *UserVolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo
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

	extraWarnings, extraErrors := s.ProvisioningSpec.Validate(true)

	warnings = append(warnings, extraWarnings...)
	validationErrors = errors.Join(validationErrors, extraErrors)

	extraWarnings, extraErrors = s.FilesystemSpec.Validate()
	warnings = append(warnings, extraWarnings...)
	validationErrors = errors.Join(validationErrors, extraErrors)

	extraWarnings, extraErrors = s.EncryptionSpec.Validate()
	warnings = append(warnings, extraWarnings...)
	validationErrors = errors.Join(validationErrors, extraErrors)

	return warnings, validationErrors
}

// UserVolumeConfigSignal is a signal for user volume config.
func (s *UserVolumeConfigV1Alpha1) UserVolumeConfigSignal() {}

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
}

// Type implements config.FilesystemConfig interface.
func (s FilesystemSpec) Type() block.FilesystemType {
	if s.FilesystemType == block.FilesystemTypeNone {
		return block.FilesystemTypeXFS
	}

	return s.FilesystemType
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

	return nil, nil
}
