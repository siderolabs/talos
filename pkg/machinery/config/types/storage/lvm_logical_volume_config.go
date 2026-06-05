// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// LVMLogicalVolumeConfigKind is a config document kind.
const LVMLogicalVolumeConfigKind = "LVMLogicalVolumeConfig"

func init() {
	registry.Register(LVMLogicalVolumeConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &LVMLogicalVolumeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.LVMLogicalVolumeConfig = &LVMLogicalVolumeConfigV1Alpha1{}
	_ config.NamedDocument          = &LVMLogicalVolumeConfigV1Alpha1{}
	_ config.Validator              = &LVMLogicalVolumeConfigV1Alpha1{}
)

// LVM2 NAME_LEN is 128; keep LV names shorter for stable resource IDs.
const maxLVMLogicalVolumeNameLength = 63

// LVMLogicalVolumeConfigV1Alpha1 is an LVM logical volume config document.
//
//	description: |
//	  Defines a logical volume provisioned inside a volume group.
//	examples:
//	  - value: exampleLVMLogicalVolumeConfigV1Alpha1()
//	alias: LVMLogicalVolumeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/LVMLogicalVolumeConfig
type LVMLogicalVolumeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Logical volume name.
	//
	//     Must be 1-63 chars: ASCII letters, digits, hyphens, underscores.
	MetaName string `yaml:"name"`
	//   description: |
	//     Logical volume layout.
	//   values:
	//     - linear
	//     - raid0
	//     - raid1
	//     - raid10
	//   schema:
	//     type: string
	LVType storageres.LVMLogicalVolumeType `yaml:"type"`
	//   description: |
	//     Number of mirror copies for `raid1` / `raid10` layouts.
	//
	//     Defaults to 1 (a two-way mirror) when unset. Not valid for `linear`
	//     or `raid0`.
	LVMirrors *uint32 `yaml:"mirrors,omitempty"`
	//   description: |
	//     Number of stripes for `raid0` / `raid10` layouts.
	//
	//     Defaults to all available physical volumes when unset. Must be at
	//     least 2. Not valid for `linear` or `raid1`.
	LVStripes *uint32 `yaml:"stripes,omitempty"`
	//   description: |
	//     Describes how the logical volume is provisioned.
	Provisioning LVMLogicalVolumeProvisioningSpec `yaml:"provisioning"`
}

// LVMLogicalVolumeProvisioningSpec describes how an LV is provisioned.
type LVMLogicalVolumeProvisioningSpec struct {
	//   description: |
	//     Name of the volume group that backs the logical volume.
	VolumeGroup string `yaml:"volumeGroup"`
	//  description: |
	//    The minimum size of the volume.
	//
	//    Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB.
	//  schema:
	//    type: string
	ProvisioningMinSize block.ByteSize `yaml:"minSize,omitempty"`
	//  description: |
	//    The maximum size of the volume.
	//
	//    Size is specified in bytes or in percents of the volume group.
	//    It can be expressed in human readable format, e.g. 100MB or 80%.
	//  schema:
	//    type: string
	ProvisioningMaxSize block.Size `yaml:"maxSize,omitempty"`
}

// NewLVMLogicalVolumeConfigV1Alpha1 creates a new LVMLogicalVolumeConfig document.
func NewLVMLogicalVolumeConfigV1Alpha1() *LVMLogicalVolumeConfigV1Alpha1 {
	return &LVMLogicalVolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       LVMLogicalVolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleLVMLogicalVolumeConfigV1Alpha1() *LVMLogicalVolumeConfigV1Alpha1 {
	cfg := NewLVMLogicalVolumeConfigV1Alpha1()
	cfg.MetaName = "lv-data"
	cfg.LVType = storageres.LVMLogicalVolumeTypeLinear
	cfg.Provisioning = LVMLogicalVolumeProvisioningSpec{
		VolumeGroup:         "vg-pool",
		ProvisioningMaxSize: block.MustSize("50GiB"),
	}

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *LVMLogicalVolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *LVMLogicalVolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// LVMLogicalVolumeConfigSignal is a marker for config.LVMLogicalVolumeConfig.
func (s *LVMLogicalVolumeConfigV1Alpha1) LVMLogicalVolumeConfigSignal() {}

// VolumeGroup implements config.LVMLogicalVolumeConfig.
func (s *LVMLogicalVolumeConfigV1Alpha1) VolumeGroup() string {
	return s.Provisioning.VolumeGroup
}

// Type implements config.LVMLogicalVolumeConfig.
func (s *LVMLogicalVolumeConfigV1Alpha1) Type() storageres.LVMLogicalVolumeType {
	return s.LVType
}

// MaxSizeBytes implements config.LVMLogicalVolumeConfig.
func (s *LVMLogicalVolumeConfigV1Alpha1) MaxSizeBytes() uint64 {
	return s.Provisioning.ProvisioningMaxSize.Value()
}

// MaxSizePercentVG implements config.LVMLogicalVolumeConfig.
func (s *LVMLogicalVolumeConfigV1Alpha1) MaxSizePercentVG() uint32 {
	if v, ok := s.Provisioning.ProvisioningMaxSize.RelativeValue(); ok {
		return uint32(v)
	}

	return 0
}

// MinSizeBytes implements config.LVMLogicalVolumeConfig.
func (s *LVMLogicalVolumeConfigV1Alpha1) MinSizeBytes() uint64 {
	return s.Provisioning.ProvisioningMinSize.Value()
}

// Mirrors implements config.LVMLogicalVolumeConfig. For mirrored layouts
// (raid1/raid10) an unset value defaults to 1 (a two-way mirror); for other
// layouts it returns 0 (not applicable).
func (s *LVMLogicalVolumeConfigV1Alpha1) Mirrors() uint32 {
	if s.LVMirrors != nil {
		return *s.LVMirrors
	}

	switch s.LVType {
	case storageres.LVMLogicalVolumeTypeRAID1, storageres.LVMLogicalVolumeTypeRAID10:
		return 1
	case storageres.LVMLogicalVolumeTypeLinear, storageres.LVMLogicalVolumeTypeRAID0:
		fallthrough
	default:
		return 0
	}
}

// Stripes implements config.LVMLogicalVolumeConfig. An unset value returns 0,
// which the reconcile controller resolves to all available physical volumes.
func (s *LVMLogicalVolumeConfigV1Alpha1) Stripes() uint32 {
	if s.LVStripes != nil {
		return *s.LVStripes
	}

	return 0
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo,cyclop
func (s *LVMLogicalVolumeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	if s.MetaName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	}

	if len(s.MetaName) < 1 || len(s.MetaName) > maxLVMLogicalVolumeNameLength {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("name must be between 1 and %d characters long", maxLVMLogicalVolumeNameLength))
	}

	if strings.ContainsFunc(s.MetaName, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= 'A' && r <= 'Z':
			return false
		case r >= '0' && r <= '9':
			return false
		case r == '-' || r == '_':
			return false
		default:
			return true
		}
	}) {
		validationErrors = errors.Join(validationErrors, errors.New("name can only contain ASCII letters, digits, hyphens and underscores"))
	}

	// LVType is a typed enum; unsupported string values are rejected at decode
	// time by its TextUnmarshaler. Here we only check that mirrors/stripes are
	// set in combinations that make sense for the chosen layout.
	usesMirrors := s.LVType == storageres.LVMLogicalVolumeTypeRAID1 || s.LVType == storageres.LVMLogicalVolumeTypeRAID10
	usesStripes := s.LVType == storageres.LVMLogicalVolumeTypeRAID0 || s.LVType == storageres.LVMLogicalVolumeTypeRAID10

	if s.LVMirrors != nil {
		if !usesMirrors {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("mirrors is only valid for raid1/raid10, not %s", s.LVType))
		} else if *s.LVMirrors < 1 {
			validationErrors = errors.Join(validationErrors, errors.New("mirrors must be at least 1"))
		}
	}

	if s.LVStripes != nil {
		if !usesStripes {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("stripes is only valid for raid0/raid10, not %s", s.LVType))
		} else if *s.LVStripes < 2 {
			validationErrors = errors.Join(validationErrors, errors.New("stripes must be at least 2"))
		}
	}

	if s.Provisioning.VolumeGroup == "" {
		validationErrors = errors.Join(validationErrors, errors.New("provisioning.volumeGroup is required"))
	}

	if s.Provisioning.ProvisioningMaxSize.IsZero() {
		validationErrors = errors.Join(validationErrors, errors.New("provisioning.maxSize is required"))
	}

	if s.Provisioning.ProvisioningMaxSize.IsNegative() {
		validationErrors = errors.Join(validationErrors, errors.New("provisioning.maxSize must not be negative"))
	}

	// When both sizes are absolute, minSize must not exceed maxSize.
	if !s.Provisioning.ProvisioningMinSize.IsZero() &&
		!s.Provisioning.ProvisioningMaxSize.IsRelative() &&
		s.Provisioning.ProvisioningMinSize.Value() > s.Provisioning.ProvisioningMaxSize.Value() {
		validationErrors = errors.Join(validationErrors, errors.New("provisioning.minSize must not exceed provisioning.maxSize"))
	}

	return nil, validationErrors
}
