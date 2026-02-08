// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// LVMVolumeGroupKind is a config document kind.
const LVMVolumeGroupKind = "LVMVolumeGroupConfig"

func init() {
	registry.Register(LVMVolumeGroupKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &LVMVolumeGroupV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces
var (
	_ config.NamedDocument = &LVMVolumeGroupV1Alpha1{}
	_ config.Validator     = &LVMVolumeGroupV1Alpha1{}
)

var vgNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

// LVMVolumeGroupV1Alpha1 represents an LVM volume group configuration.
//
//	description: |
//		LVMVolumeGroupConfig defines an LVM Volume Group composed from one or more
//		physical volumes. Physical volumes can be selected using a CEL expression.
//	examples:
//	  - value: exampleLVMVolumeGroupV1Alpha1()
//	alias: LVMVolumeGroup
//	schemaRoot: true
//	schemaMeta: v1alpha1/LVMVolumeGroup
type LVMVolumeGroupV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the volume group.
	MetaName string `yaml:"name"`

	//   description: |
	//     Specification of physical volumes that belong to this volume group.
	PhysicalVolumes PhysicalVolumeSpec `yaml:"physicalVolumes,omitempty"`
}

// PhysicalVolumeSpec defines how physical volumes are specified.
type PhysicalVolumeSpec struct {
	//   description: |
	//     Selector to dynamically select physical volumes based on attributes.
	VolumeSelector VolumeSelector `yaml:"volumeSelector,omitempty"`
}

// IsZero checks if the PhysicalVolumeSpec is zero.
func (s PhysicalVolumeSpec) IsZero() bool {
	return s.VolumeSelector.Match.IsZero()
}

// NewLVMVolumeGroupV1Alpha1 creates a new LVMVolumeGroup config document.
func NewLVMVolumeGroupV1Alpha1() *LVMVolumeGroupV1Alpha1 {
	return &LVMVolumeGroupV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       LVMVolumeGroupKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleLVMVolumeGroupV1Alpha1() *LVMVolumeGroupV1Alpha1 {
	cfg := NewLVMVolumeGroupV1Alpha1()
	cfg.MetaName = "vg-pool"
	cfg.PhysicalVolumes = PhysicalVolumeSpec{
		VolumeSelector: VolumeSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`disk.dev_path == "/dev/sda0"`, celenv.DiskLocator())),
		},
	}
	return cfg
}

func exampleLVMDiskSelector1() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`disk.dev_path == "/dev/sda0"`, celenv.DiskLocator()))
}

// Name implements config.NamedDocument interface.
func (l *LVMVolumeGroupV1Alpha1) Name() string {
	return l.MetaName
}

// Clone implements config.Document interface.
func (l *LVMVolumeGroupV1Alpha1) Clone() config.Document { //nolint:wrapcheck
	return l.DeepCopy()
}

// Validate implements config.Validator.
func (l *LVMVolumeGroupV1Alpha1) Validate(_ validation.RuntimeMode, _ ...validation.Option) ([]string, error) {
	var validationErrors error

	if l.MetaName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	} else if !vgNameRegex.MatchString(l.MetaName) {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("invalid volume group name: %q", l.MetaName))
	}

	if !l.PhysicalVolumes.VolumeSelector.Match.IsZero() {
		if err := l.PhysicalVolumes.VolumeSelector.Match.ParseBool(celenv.DiskLocator()); err != nil {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("volume selector is invalid: %w", err))
		}
	} else {
		validationErrors = errors.Join(validationErrors, errors.New("physicalVolumes.volumeSelector.match is required"))
	}

	return nil, validationErrors
}
