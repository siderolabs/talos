// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// LVMLogicalVolumeKind is a config document kind.
const LVMLogicalVolumeKind = "LVMLogicalVolumeConfig"

func init() {
	registry.Register(LVMLogicalVolumeKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &LVMLogicalVolumeV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces
var (
	_ config.NamedDocument = &LVMLogicalVolumeV1Alpha1{}
	_ config.Validator     = &LVMLogicalVolumeV1Alpha1{}
)

var lvNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

// LVMLogicalVolumeV1Alpha1 represents an LVM logical volume configuration.
//
//	description: |
//		LVMLogicalVolumeConfig defines an LVM Logical Volume within a Volume Group.
//	examples:
//	  - value: exampleLVMLogicalVolumeV1Alpha1()
//	alias: LVMLogicalVolume
//	schemaRoot: true
//	schemaMeta: v1alpha1/LVMLogicalVolume
type LVMLogicalVolumeV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the logical volume.
	MetaName string `yaml:"name"`

	//   description: |
	//     Type of the logical volume.
	//   values:
	//     - linear
	//   schema:
	//     type: string
	LVType string `yaml:"lvType,omitempty"`

	//   description: |
	//     Provisioning describes how the logical volume is provisioned.
	Provisioning LVMLogicalVolumeProvisioning `yaml:"provisioning,omitempty"`
}

// LVMLogicalVolumeProvisioning defines provisioning settings for a logical volume.
type LVMLogicalVolumeProvisioning struct {
	//   description: |
	//     Specifies who manages this logical volume.
	//   values:
	//     - talos
	//     - csi
	//   schema:
	//     type: string
	ManagedBy string `yaml:"managedBy,omitempty"`

	//   description: |
	//     Name of the volume group this logical volume belongs to.
	VolumeGroup string `yaml:"volumeGroup,omitempty"`

	//   description: |
	//     The minimum size of the logical volume.
	//
	//     Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB.
	//   examples:
	//     - value: >
	//         "2.5GiB"
	//   schema:
	//     type: string
	MinSize ByteSize `yaml:"minSize,omitempty"`

	//   description: |
	//     The maximum size of the logical volume.
	//     If not specified, the logical volume will use 100% of the free space in the volume group.
	//
	//     Size is specified in bytes or in percents. It can be expressed in human readable format, e.g. 100MB.
	//   examples:
	//     - value: >
	//         "50GiB"
	//     - value: >
	//         "80%"
	//   schema:
	//     type: string
	MaxSize Size `yaml:"maxSize,omitempty"`
}

// IsZero checks if the LVM logical volume provisioning is zero.
func (s LVMLogicalVolumeProvisioning) IsZero() bool {
	return s.ManagedBy == "" && s.VolumeGroup == "" && s.MinSize.IsZero() && s.MaxSize.IsZero()
}

// NewLVMLogicalVolumeV1Alpha1 creates a new LVMLogicalVolume config document.
func NewLVMLogicalVolumeV1Alpha1() *LVMLogicalVolumeV1Alpha1 {
	return &LVMLogicalVolumeV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       LVMLogicalVolumeKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleLVMLogicalVolumeV1Alpha1() *LVMLogicalVolumeV1Alpha1 {
	cfg := NewLVMLogicalVolumeV1Alpha1()
	cfg.MetaName = "lv-data"
	cfg.LVType = "linear"
	cfg.Provisioning = LVMLogicalVolumeProvisioning{
		ManagedBy:   "talos",
		VolumeGroup: "vg-pool",
		MaxSize:     MustSize("50GiB"),
	}
	return cfg
}

// Name implements config.NamedDocument interface.
func (l *LVMLogicalVolumeV1Alpha1) Name() string {
	return l.MetaName
}

// Clone implements config.Document interface.
func (l *LVMLogicalVolumeV1Alpha1) Clone() config.Document { //nolint:wrapcheck
	return l.DeepCopy()
}

// Validate implements config.Validator.
func (l *LVMLogicalVolumeV1Alpha1) Validate(_ validation.RuntimeMode, _ ...validation.Option) ([]string, error) {
	var validationErrors error

	if l.MetaName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	} else if !lvNameRegex.MatchString(l.MetaName) {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("invalid logical volume name: %q", l.MetaName))
	}

	if l.LVType == "" {
		validationErrors = errors.Join(validationErrors, errors.New("lvType is required"))
	} else {
		validLVTypes := []string{"linear", "striped", "raid1"}
		isValid := false
		for _, vt := range validLVTypes {
			if l.LVType == vt {
				isValid = true
				break
			}
		}
		if !isValid {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("invalid lvType: %q, must be one of: linear, striped, raid1", l.LVType))
		}
	}

	if l.Provisioning.ManagedBy != "" {
		validManagedBy := []string{"talos", "csi"}
		isValid := false
		for _, mb := range validManagedBy {
			if l.Provisioning.ManagedBy == mb {
				isValid = true
				break
			}
		}
		if !isValid {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("invalid provisioning.managedBy: %q, must be one of: talos, csi", l.Provisioning.ManagedBy))
		}
	}

	if l.Provisioning.VolumeGroup == "" {
		validationErrors = errors.Join(validationErrors, errors.New("provisioning.volumeGroup is required"))
	} else if !vgNameRegex.MatchString(l.Provisioning.VolumeGroup) {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("invalid volume group name: %q", l.Provisioning.VolumeGroup))
	}

	if !l.Provisioning.MinSize.IsZero() && !l.Provisioning.MaxSize.IsZero() && !l.Provisioning.MaxSize.IsRelative() {
		if l.Provisioning.MinSize.Value() > l.Provisioning.MaxSize.Value() {
			validationErrors = errors.Join(validationErrors, errors.New("provisioning.minSize is greater than provisioning.maxSize"))
		}
	}

	return nil, validationErrors
}
