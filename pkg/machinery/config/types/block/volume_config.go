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

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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
	_ config.VolumeConfig  = &VolumeConfigV1Alpha1{}
	_ config.NamedDocument = &VolumeConfigV1Alpha1{}
	_ config.Validator     = &VolumeConfigV1Alpha1{}
)

// VolumeConfigV1Alpha1 is a system volume configuration document.
//
//	description: |
//	  Note: at the moment, only `EPHEMERAL` and `IMAGE-CACHE` system volumes are supported.
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
	allowedVolumes := []string{constants.EphemeralPartitionLabel, constants.ImageCachePartitionLabel}

	if slices.Index(allowedVolumes, s.MetaName) == -1 {
		return nil, fmt.Errorf("only %q volumes are supported", allowedVolumes)
	}

	return s.ProvisioningSpec.Validate(false)
}

// Provisioning implements config.VolumeConfig interface.
func (s *VolumeConfigV1Alpha1) Provisioning() config.VolumeProvisioningConfig {
	return s.ProvisioningSpec
}

// Validate the provisioning spec.
func (s ProvisioningSpec) Validate(required bool) ([]string, error) {
	var validationErrors error

	if !s.DiskSelectorSpec.Match.IsZero() {
		if err := s.DiskSelectorSpec.Match.ParseBool(celenv.DiskLocator()); err != nil {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("disk selector is invalid: %w", err))
		}
	} else if required {
		validationErrors = errors.Join(validationErrors, errors.New("disk selector is required"))
	}

	if !s.ProvisioningMinSize.IsZero() && !s.ProvisioningMaxSize.IsZero() {
		if s.ProvisioningMinSize.Value() > s.ProvisioningMaxSize.Value() {
			validationErrors = errors.Join(validationErrors, errors.New("min size is greater than max size"))
		}
	} else if required && s.ProvisioningMinSize.IsZero() && s.ProvisioningMaxSize.IsZero() {
		validationErrors = errors.Join(validationErrors, errors.New("min size or max size is required"))
	}

	return nil, validationErrors
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
