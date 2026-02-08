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

// RawVolumeConfigKind is a config document kind.
const RawVolumeConfigKind = "RawVolumeConfig"

func init() {
	registry.Register(RawVolumeConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &RawVolumeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.RawVolumeConfig = &RawVolumeConfigV1Alpha1{}
	_ config.NamedDocument   = &RawVolumeConfigV1Alpha1{}
	_ config.Validator       = &RawVolumeConfigV1Alpha1{}
)

const maxRawVolumeNameLength = constants.PartitionLabelLength - len(constants.RawVolumePrefix)

// RawVolumeConfigV1Alpha1 is a raw volume configuration document.
//
//		description: |
//		  Raw volumes allow to create partitions without formatting them.
//	   If you want to use local storage, user volumes is a better choice,
//	   raw volumes are intended to be used with CSI provisioners.
//		  The partition label is automatically generated as `r-<name>`.
//		examples:
//		  - value: exampleRawVolumeConfigV1Alpha1()
//		alias: RawVolumeConfig
//		schemaRoot: true
//		schemaMeta: v1alpha1/RawVolumeConfig
type RawVolumeConfigV1Alpha1 struct {
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
	//     The encryption describes how the volume is encrypted.
	EncryptionSpec EncryptionSpec `yaml:"encryption,omitempty"`
}

// NewRawVolumeConfigV1Alpha1 creates a new raw volume config document.
func NewRawVolumeConfigV1Alpha1() *RawVolumeConfigV1Alpha1 {
	return &RawVolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       RawVolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleRawVolumeConfigV1Alpha1() *RawVolumeConfigV1Alpha1 {
	cfg := NewRawVolumeConfigV1Alpha1()
	cfg.MetaName = "local-data"
	cfg.ProvisioningSpec = ProvisioningSpec{
		DiskSelectorSpec: DiskSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "nvme"`, celenv.DiskLocator())),
		},
		ProvisioningMaxSize: MustSize("50GiB"),
	}

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *RawVolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *RawVolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo,dupl
func (s *RawVolumeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		warnings         []string //nolint:prealloc
		validationErrors error
	)

	if s.MetaName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	}

	if len(s.MetaName) < 1 || len(s.MetaName) > maxRawVolumeNameLength {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("name must be between 1 and %d characters long", maxRawVolumeNameLength))
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

	extraWarnings, extraErrors := s.ProvisioningSpec.Validate(true, true)

	warnings = append(warnings, extraWarnings...)
	validationErrors = errors.Join(validationErrors, extraErrors)

	extraWarnings, extraErrors = s.EncryptionSpec.Validate()
	warnings = append(warnings, extraWarnings...)
	validationErrors = errors.Join(validationErrors, extraErrors)

	return warnings, validationErrors
}

// RawVolumeConfigSignal is a signal for user volume config.
func (s *RawVolumeConfigV1Alpha1) RawVolumeConfigSignal() {}

// Provisioning implements config.RawVolumeConfig interface.
func (s *RawVolumeConfigV1Alpha1) Provisioning() config.VolumeProvisioningConfig {
	return s.ProvisioningSpec
}

// Encryption implements config.RawVolumeConfig interface.
func (s *RawVolumeConfigV1Alpha1) Encryption() config.EncryptionConfig {
	if s.EncryptionSpec.EncryptionProvider == block.EncryptionProviderNone {
		return nil
	}

	return s.EncryptionSpec
}
