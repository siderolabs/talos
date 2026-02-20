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

// LVMPhysicalVolumeKind is a config document kind.
const LVMPhysicalVolumeKind = "LVMPhysicalVolumeConfig"

func init() {
	registry.Register(LVMPhysicalVolumeKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &LVMPhysicalVolumeV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces
var (
	_ config.NamedDocument = &LVMPhysicalVolumeV1Alpha1{}
	_ config.Validator     = &LVMPhysicalVolumeV1Alpha1{}
)

var pvNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

// LVMPhysicalVolumeV1Alpha1 represents an LVM physical volume configuration.
//
//	description: |
//		LVMPhysicalVolumeConfig defines an LVM Physical Volume created from a disk or partition.
//		Physical volumes are selected using a CEL expression.
//	examples:
//	  - value: exampleLVMPhysicalVolumeV1Alpha1()
//	alias: LVMPhysicalVolume
//	schemaRoot: true
//	schemaMeta: v1alpha1/LVMPhysicalVolume
type LVMPhysicalVolumeV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the physical volume.
	MetaName string `yaml:"name"`

	//   description: |
	//     Selector to dynamically select a disk or partition for the physical volume.
	DeviceSelector VolumeSelector `yaml:"deviceSelector,omitempty"`
}

// IsZero checks if the LVMPhysicalVolume device selector is zero.
func (l LVMPhysicalVolumeV1Alpha1) IsZero() bool {
	return l.DeviceSelector.Match.IsZero()
}

// NewLVMPhysicalVolumeV1Alpha1 creates a new LVMPhysicalVolume config document.
func NewLVMPhysicalVolumeV1Alpha1() *LVMPhysicalVolumeV1Alpha1 {
	return &LVMPhysicalVolumeV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       LVMPhysicalVolumeKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleLVMPhysicalVolumeV1Alpha1() *LVMPhysicalVolumeV1Alpha1 {
	cfg := NewLVMPhysicalVolumeV1Alpha1()
	cfg.MetaName = "pv1"
	cfg.DeviceSelector = VolumeSelector{
		Match: cel.MustExpression(cel.ParseBooleanExpression(`disk.dev_path == "/dev/sda0"`, celenv.DiskLocator())),
	}
	return cfg
}

// Name implements config.NamedDocument interface.
func (l *LVMPhysicalVolumeV1Alpha1) Name() string {
	return l.MetaName
}

// Clone implements config.Document interface.
func (l *LVMPhysicalVolumeV1Alpha1) Clone() config.Document { //nolint:wrapcheck
	return l.DeepCopy()
}

// Validate implements config.Validator.
func (l *LVMPhysicalVolumeV1Alpha1) Validate(_ validation.RuntimeMode, _ ...validation.Option) ([]string, error) {
	var validationErrors error

	if l.MetaName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	} else if !pvNameRegex.MatchString(l.MetaName) {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("invalid physical volume name: %q", l.MetaName))
	}

	if !l.DeviceSelector.Match.IsZero() {
		if err := l.DeviceSelector.Match.ParseBool(celenv.DiskLocator()); err != nil {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("device selector is invalid: %w", err))
		}
	} else {
		validationErrors = errors.Join(validationErrors, errors.New("deviceSelector.match is required"))
	}

	return nil, validationErrors
}
