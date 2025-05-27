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

// SwapVolumeConfigKind is a config document kind.
const SwapVolumeConfigKind = "SwapVolumeConfig"

func init() {
	registry.Register(SwapVolumeConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &SwapVolumeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.SwapVolumeConfig = &SwapVolumeConfigV1Alpha1{}
	_ config.NamedDocument    = &SwapVolumeConfigV1Alpha1{}
	_ config.Validator        = &SwapVolumeConfigV1Alpha1{}
)

const maxSwapVolumeNameLength = constants.PartitionLabelLength - len(constants.SwapVolumePrefix)

// SwapVolumeConfigV1Alpha1 is a disk swap volume configuration document.
//
//	description: |
//	  Swap volume is automatically allocated as a partition on the specified disk
//	  and activated as swap, removing a swap volume deactivates swap.
//	  The partition label is automatically generated as `s-<name>`.
//	examples:
//	  - value: exampleSwapVolumeConfigV1Alpha1()
//	alias: SwapVolumeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/SwapVolumeConfig
type SwapVolumeConfigV1Alpha1 struct {
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

// NewSwapVolumeConfigV1Alpha1 creates a new user volume config document.
func NewSwapVolumeConfigV1Alpha1() *SwapVolumeConfigV1Alpha1 {
	return &SwapVolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       SwapVolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleSwapVolumeConfigV1Alpha1() *SwapVolumeConfigV1Alpha1 {
	cfg := NewSwapVolumeConfigV1Alpha1()
	cfg.MetaName = "swap1"
	cfg.ProvisioningSpec = ProvisioningSpec{
		DiskSelectorSpec: DiskSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "nvme"`, celenv.DiskLocator())),
		},
		ProvisioningMinSize: MustByteSize("3GiB"),
		ProvisioningMaxSize: MustByteSize("4GiB"),
	}
	cfg.EncryptionSpec = EncryptionSpec{
		EncryptionProvider: block.EncryptionProviderLUKS2,
		EncryptionKeys: []EncryptionKey{
			{
				KeySlot: 0,
				KeyStatic: &EncryptionKeyStatic{
					KeyData: "swapsecret",
				},
			},
		},
	}

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *SwapVolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *SwapVolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo
func (s *SwapVolumeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		warnings         []string
		validationErrors error
	)

	if s.MetaName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	}

	if len(s.MetaName) < 1 || len(s.MetaName) > maxSwapVolumeNameLength {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("name must be between 1 and %d characters long", maxSwapVolumeNameLength))
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

	extraWarnings, extraErrors = s.EncryptionSpec.Validate()
	warnings = append(warnings, extraWarnings...)
	validationErrors = errors.Join(validationErrors, extraErrors)

	return warnings, validationErrors
}

// SwapVolumeConfigSignal is a signal for swap volume config.
func (s *SwapVolumeConfigV1Alpha1) SwapVolumeConfigSignal() {}

// Provisioning implements config.SwapVolumeConfig interface.
func (s *SwapVolumeConfigV1Alpha1) Provisioning() config.VolumeProvisioningConfig {
	return s.ProvisioningSpec
}

// Encryption implements config.SwapVolumeConfig interface.
func (s *SwapVolumeConfigV1Alpha1) Encryption() config.EncryptionConfig {
	if s.EncryptionSpec.EncryptionProvider == block.EncryptionProviderNone {
		return nil
	}

	return s.EncryptionSpec
}
