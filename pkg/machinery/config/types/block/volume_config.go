// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"slices"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// VolumeConfigKind is a config document kind.
const VolumeConfigKind = "VolumeConfig"

func init() {
	registry.Register(VolumeConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &VolumeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.VolumeConfig                 = &VolumeConfigV1Alpha1{}
	_ config.NamedDocument                = &VolumeConfigV1Alpha1{}
	_ config.Validator                    = &VolumeConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &VolumeConfigV1Alpha1{}
)

// VolumeConfigV1Alpha1 is a system volume configuration document.
//
//	description: |
//	  Note: at the moment, only `STATE`, `EPHEMERAL` and `IMAGE-CACHE` system volumes are supported.
//	examples:
//	  - value: exampleVolumeConfigEphemeralV1Alpha1()
//	alias: VolumeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/VolumeConfig
type VolumeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the volume.
	MetaName string `yaml:"name"`
	//   description: |
	//     The provisioning describes how the volume is provisioned.
	ProvisioningSpec ProvisioningSpec `yaml:"provisioning,omitempty"`
	//   description: |
	//     The encryption describes how the volume is encrypted.
	EncryptionSpec EncryptionSpec `yaml:"encryption,omitempty"`
}

// ProvisioningSpec describes how the volume is provisioned.
type ProvisioningSpec struct {
	//   description: |
	//     The disk selector expression.
	DiskSelectorSpec DiskSelector `yaml:"diskSelector,omitempty"`
	//   description: |
	//    Should the volume grow to the size of the disk (if possible).
	ProvisioningGrow *bool `yaml:"grow,omitempty"`
	//  description: |
	//    The minimum size of the volume.
	//
	//    Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB.
	//  examples:
	//    - value: >
	//        "2.5GiB"
	//  schema:
	//    type: string
	ProvisioningMinSize ByteSize `yaml:"minSize,omitempty"`
	//  description: |
	//    The maximum size of the volume, if not specified the volume can grow to the size of the
	//    disk.
	//
	//    Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB.
	//  examples:
	//    - value: >
	//        "50GiB"
	//  schema:
	//    type: string
	ProvisioningMaxSize ByteSize `yaml:"maxSize,omitempty"`
}

// DiskSelector selects a disk for the volume.
type DiskSelector struct {
	//   description: |
	//     The Common Expression Language (CEL) expression to match the disk.
	//   schema:
	//     type: string
	//   examples:
	//    - value: >
	//        exampleDiskSelector1()
	//      name: match disks with size between 120GB and 1TB
	//    - value: >
	//        exampleDiskSelector2()
	//      name: match SATA disks that are not rotational and not system disks
	Match cel.Expression `yaml:"match,omitempty"`
}

// NewVolumeConfigV1Alpha1 creates a new volume config document.
func NewVolumeConfigV1Alpha1() *VolumeConfigV1Alpha1 {
	return &VolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       VolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleVolumeConfigEphemeralV1Alpha1() *VolumeConfigV1Alpha1 {
	cfg := NewVolumeConfigV1Alpha1()
	cfg.MetaName = constants.EphemeralPartitionLabel
	cfg.ProvisioningSpec = ProvisioningSpec{
		DiskSelectorSpec: DiskSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "nvme"`, celenv.DiskLocator())),
		},
		ProvisioningMaxSize: MustByteSize("50GiB"),
	}

	return cfg
}

func exampleDiskSelector1() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`disk.size > 120u * GB && disk.size < 1u * TB`, celenv.DiskLocator()))
}

func exampleDiskSelector2() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "sata" && !disk.rotational && !system_disk`, celenv.DiskLocator()))
}

// Name implements config.NamedDocument interface.
func (s *VolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *VolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *VolumeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	allowedVolumes := []string{
		constants.StatePartitionLabel,
		constants.EphemeralPartitionLabel,
		constants.ImageCachePartitionLabel,
	}

	if slices.Index(allowedVolumes, s.MetaName) == -1 {
		return nil, fmt.Errorf("only %q volumes are supported", allowedVolumes)
	}

	var (
		warnings         []string
		validationErrors error
	)

	if s.MetaName == constants.StatePartitionLabel {
		// no provisioning config is allowed for the state partition.
		if !s.ProvisioningSpec.IsZero() {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("provisioning config is not allowed for the %q volume", s.MetaName))
		}

		for _, key := range s.EncryptionSpec.EncryptionKeys {
			if pointer.SafeDeref(key.KeyLockToSTATE) {
				// state-locked keys are not allowed
				validationErrors = errors.Join(validationErrors, fmt.Errorf("state-locked key is not allowed for the %q volume", s.MetaName))
			}
		}
	}

	extraWarnings, extraErrors := s.ProvisioningSpec.Validate(false, false)
	warnings = append(warnings, extraWarnings...)
	validationErrors = errors.Join(validationErrors, extraErrors)

	extraWarnings, extraErrors = s.EncryptionSpec.Validate()
	warnings = append(warnings, extraWarnings...)
	validationErrors = errors.Join(validationErrors, extraErrors)

	return warnings, validationErrors
}

// Provisioning implements config.VolumeConfig interface.
func (s *VolumeConfigV1Alpha1) Provisioning() config.VolumeProvisioningConfig {
	return s.ProvisioningSpec
}

// Encryption implements config.VolumeConfig interface.
func (s *VolumeConfigV1Alpha1) Encryption() config.EncryptionConfig {
	if s.EncryptionSpec.EncryptionProvider == block.EncryptionProviderNone {
		return nil
	}

	return s.EncryptionSpec
}

// Validate the provisioning spec.
//
//nolint:gocyclo
func (s ProvisioningSpec) Validate(required, sizeSupported bool) ([]string, error) {
	var validationErrors error

	if !s.DiskSelectorSpec.Match.IsZero() {
		if err := s.DiskSelectorSpec.Match.ParseBool(celenv.DiskLocator()); err != nil {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("disk selector is invalid: %w", err))
		}
	} else if required {
		validationErrors = errors.Join(validationErrors, errors.New("disk selector is required"))
	}

	switch {
	case sizeSupported && !s.ProvisioningMinSize.IsZero() && !s.ProvisioningMaxSize.IsZero():
		if s.ProvisioningMinSize.Value() > s.ProvisioningMaxSize.Value() {
			validationErrors = errors.Join(validationErrors, errors.New("min size is greater than max size"))
		}

	case required && sizeSupported && s.ProvisioningMinSize.IsZero() && s.ProvisioningMaxSize.IsZero():
		validationErrors = errors.Join(validationErrors, errors.New("min size or max size is required"))

	case required && !sizeSupported:
		if !s.ProvisioningMinSize.IsZero() || !s.ProvisioningMaxSize.IsZero() || s.Grow().IsPresent() {
			validationErrors = errors.Join(validationErrors, errors.New("min size, max size and grow are not supported"))
		}
	}

	return nil, validationErrors
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *VolumeConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Config *v1alpha1.Config) error {
	if !slices.Contains([]string{constants.StatePartitionLabel, constants.EphemeralPartitionLabel}, s.MetaName) {
		// only STATE and EPHEMERAL volumes can conflict with legacy config.
		return nil
	}

	if s.Encryption() == nil {
		// no encryption configured, no conflict.
		return nil
	}

	legacy := v1alpha1Config.Machine().SystemDiskEncryption().Get(s.MetaName)
	if legacy != nil {
		return fmt.Errorf("system disk encryption for %q is configured in both v1alpha1.Config and VolumeConfig", s.MetaName)
	}

	return nil
}

// IsZero checks if the provisioning spec is zero.
func (s ProvisioningSpec) IsZero() bool {
	return s.ProvisioningGrow == nil && s.ProvisioningMaxSize.IsZero() && s.ProvisioningMinSize.IsZero() && s.DiskSelectorSpec.Match.IsZero()
}

// DiskSelector implements config.VolumeProvisioningConfig interface.
func (s ProvisioningSpec) DiskSelector() optional.Optional[cel.Expression] {
	if s.DiskSelectorSpec.Match.IsZero() {
		return optional.None[cel.Expression]()
	}

	return optional.Some(s.DiskSelectorSpec.Match)
}

// Grow implements config.VolumeProvisioningConfig interface.
func (s ProvisioningSpec) Grow() optional.Optional[bool] {
	if s.ProvisioningGrow == nil {
		return optional.None[bool]()
	}

	return optional.Some(*s.ProvisioningGrow)
}

// MinSize implements config.VolumeProvisioningConfig interface.
func (s ProvisioningSpec) MinSize() optional.Optional[uint64] {
	if s.ProvisioningMinSize.IsZero() {
		return optional.None[uint64]()
	}

	return optional.Some(s.ProvisioningMinSize.Value())
}

// MaxSize implements config.VolumeProvisioningConfig interface.
func (s ProvisioningSpec) MaxSize() optional.Optional[uint64] {
	if s.ProvisioningMaxSize.IsZero() {
		return optional.None[uint64]()
	}

	return optional.Some(s.ProvisioningMaxSize.Value())
}
