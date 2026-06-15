// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

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
)

// LVMVolumeGroupConfigKind is a config document kind.
const LVMVolumeGroupConfigKind = "LVMVolumeGroupConfig"

func init() {
	registry.Register(LVMVolumeGroupConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &LVMVolumeGroupConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.LVMVolumeGroupConfig = &LVMVolumeGroupConfigV1Alpha1{}
	_ config.NamedDocument        = &LVMVolumeGroupConfigV1Alpha1{}
	_ config.Validator            = &LVMVolumeGroupConfigV1Alpha1{}
)

// LVM2 NAME_LEN is 128; keep VG names shorter for stable resource IDs.
const maxLVMVolumeGroupNameLength = 63

// LVMVolumeGroupConfigV1Alpha1 is an LVM volume group config document.
//
//	description: |
//	  Defines volume group and selector for backing disks.
//	examples:
//	  - value: exampleLVMVolumeGroupConfigV1Alpha1()
//	alias: LVMVolumeGroupConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/LVMVolumeGroupConfig
type LVMVolumeGroupConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Volume group name.
	//
	//     Must be 1-63 chars: ASCII letters, digits, hyphens, underscores.
	MetaName string `yaml:"name"`
	//   description: |
	//     The provisioning describes how the Physical Volumes are provisioned.
	ProvisioningSpec ProvisioningSpec `yaml:"provisioning"`
}

// ProvisioningSpec describes how the Physical Volumes are provisioned.
type ProvisioningSpec struct {
	//   description: |
	//     Matches disks to initialize as physical volumes.
	VolumeSelector LVMVolumeSelectorSpec `yaml:"volumeSelector,omitempty"`
}

// IsZero reports whether the spec is empty.
func (s ProvisioningSpec) IsZero() bool {
	return s.VolumeSelector.IsZero()
}

// Validate parses selector without mutating stored config.
func (s ProvisioningSpec) Validate() error {
	if s.VolumeSelector.Match.IsZero() {
		return errors.New("provisioning.volumeSelector.match is required")
	}

	if err := s.VolumeSelector.Match.ParseBool(celenv.VolumeLocator()); err != nil {
		return fmt.Errorf("provisioning.volumeSelector.match: %w", err)
	}

	return nil
}

// LVMVolumeSelectorSpec matches disks with CEL.
type LVMVolumeSelectorSpec struct {
	//   description: |
	//     CEL expression matching a disk or partition to use as a physical volume.
	//
	//     The expression is evaluated against each discovered volume with the
	//     `volume` variable (the discovered volume) and, for whole disks, the
	//     `disk` variable. Partitions (e.g. raw volumes) can be matched by their
	//     partition label via `volume.partition_label`.
	//   schema:
	//     type: string
	//   examples:
	//     - value: >
	//        exampleLVMVolumeSelector()
	//       name: match raw volume partitions labeled r-lvm*
	Match cel.Expression `yaml:"match,omitempty"`
}

// IsZero reports whether the selector is empty.
func (s LVMVolumeSelectorSpec) IsZero() bool {
	return s.Match.IsZero()
}

// NewLVMVolumeGroupConfigV1Alpha1 creates a new LVMVolumeGroupConfig document.
func NewLVMVolumeGroupConfigV1Alpha1() *LVMVolumeGroupConfigV1Alpha1 {
	return &LVMVolumeGroupConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       LVMVolumeGroupConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleLVMVolumeGroupConfigV1Alpha1() *LVMVolumeGroupConfigV1Alpha1 {
	cfg := NewLVMVolumeGroupConfigV1Alpha1()
	cfg.MetaName = "vg-pool"
	cfg.ProvisioningSpec = ProvisioningSpec{
		VolumeSelector: LVMVolumeSelectorSpec{
			Match: exampleLVMVolumeSelector(),
		},
	}

	return cfg
}

func exampleLVMVolumeSelector() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`volume.partition_label.startsWith("r-lvm")`, celenv.VolumeLocator()))
}

// Name implements config.NamedDocument interface.
func (s *LVMVolumeGroupConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *LVMVolumeGroupConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// LVMVolumeGroupConfigSignal is a marker for config.LVMVolumeGroupConfig.
func (s *LVMVolumeGroupConfigV1Alpha1) LVMVolumeGroupConfigSignal() {}

// PhysicalVolumeSelector implements config.LVMVolumeGroupConfig.
func (s *LVMVolumeGroupConfigV1Alpha1) PhysicalVolumeSelector() cel.Expression {
	return s.ProvisioningSpec.VolumeSelector.Match
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo
func (s *LVMVolumeGroupConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	if s.MetaName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	}

	if len(s.MetaName) < 1 || len(s.MetaName) > maxLVMVolumeGroupNameLength {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("name must be between 1 and %d characters long", maxLVMVolumeGroupNameLength))
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

	if err := s.ProvisioningSpec.Validate(); err != nil {
		validationErrors = errors.Join(validationErrors, err)
	}

	return nil, validationErrors
}
