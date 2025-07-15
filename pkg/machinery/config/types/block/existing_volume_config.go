// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"strings"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// ExistingVolumeConfigKind is a config document kind.
const ExistingVolumeConfigKind = "ExistingVolumeConfig"

func init() {
	registry.Register(ExistingVolumeConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &ExistingVolumeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.ExistingVolumeConfig = &ExistingVolumeConfigV1Alpha1{}
	_ config.ConflictingDocument  = &ExistingVolumeConfigV1Alpha1{}
	_ config.NamedDocument        = &ExistingVolumeConfigV1Alpha1{}
	_ config.Validator            = &ExistingVolumeConfigV1Alpha1{}
)

// ExistingVolumeConfigV1Alpha1 is an existing volume configuration document.
//
//	description: |
//	  Existing volumes allow to mount partitions (or whole disks) that were created
//	  outside of Talos. Volume will be mounted under `/var/mnt/<name>`.
//	  The existing volume config name should not conflict with user volume names.
//	examples:
//	  - value: exampleExistingVolumeConfigV1Alpha1()
//	alias: ExistingVolumeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/ExistingVolumeConfig
type ExistingVolumeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the volume.
	//
	//     Name can only contain:
	//     lowercase and uppercase ASCII letters, digits, and hyphens.
	MetaName string `yaml:"name"`
	//   description: |
	//     The discovery describes how to find a volume.
	VolumeDiscoverySpec VolumeDiscoverySpec `yaml:"discovery,omitempty"`
	//   description: |
	//     The mount describes additional mount options.
	MountSpec MountSpec `yaml:"mount,omitempty"`
}

// VolumeDiscoverySpec describes how the volume is discovered.
type VolumeDiscoverySpec struct {
	//   description: |
	//     The volume selector expression.
	VolumeSelectorConfig VolumeSelector `yaml:"volumeSelector,omitempty"`
}

// VolumeSelector selects an existing volume.
type VolumeSelector struct {
	//   description: |
	//     The Common Expression Language (CEL) expression to match the volume.
	//   schema:
	//     type: string
	//   examples:
	//    - value: >
	//        exampleVolumeSelector1()
	//      name: match volumes with partition label MY-DATA
	//    - value: >
	//        exampleVolumeSelector2()
	//      name: match xfs volume on disk with serial 'SERIAL123'
	Match cel.Expression `yaml:"match,omitempty"`
}

func exampleVolumeSelector1() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`volume.partition_label == "MY-DATA"`, celenv.VolumeLocator()))
}

func exampleVolumeSelector2() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`volume.name == "xfs" && disk.serial == "SERIAL123"`, celenv.VolumeLocator()))
}

// MountSpec describes how the volume is mounted.
type MountSpec struct {
	//   description: |
	//     Mount the volume read-only.
	MountReadOnly *bool `yaml:"readOnly,omitempty"`
}

// NewExistingVolumeConfigV1Alpha1 creates a new raw volume config document.
func NewExistingVolumeConfigV1Alpha1() *ExistingVolumeConfigV1Alpha1 {
	return &ExistingVolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       ExistingVolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleExistingVolumeConfigV1Alpha1() *ExistingVolumeConfigV1Alpha1 {
	cfg := NewExistingVolumeConfigV1Alpha1()
	cfg.MetaName = "my-existing-volume"
	cfg.VolumeDiscoverySpec = VolumeDiscoverySpec{
		VolumeSelectorConfig: VolumeSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`volume.partition_label == "MY-DATA"`, celenv.VolumeLocator())),
		},
	}

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *ExistingVolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *ExistingVolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *ExistingVolumeConfigV1Alpha1) ConflictsWithKinds() []string {
	return []string{UserVolumeConfigKind}
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo,dupl
func (s *ExistingVolumeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		warnings         []string
		validationErrors error
	)

	if s.MetaName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
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

	extraWarnings, extraErrors := s.VolumeDiscoverySpec.Validate(true)

	warnings = append(warnings, extraWarnings...)
	validationErrors = errors.Join(validationErrors, extraErrors)

	return warnings, validationErrors
}

// ExistingVolumeConfigSignal is a signal for user volume config.
func (s *ExistingVolumeConfigV1Alpha1) ExistingVolumeConfigSignal() {}

// VolumeDiscovery implements config.ExistingVolumeConfig interface.
func (s *ExistingVolumeConfigV1Alpha1) VolumeDiscovery() config.VolumeDiscoveryConfig {
	return s.VolumeDiscoverySpec
}

// Mount implements config.ExistingVolumeConfig interface.
func (s *ExistingVolumeConfigV1Alpha1) Mount() config.VolumeMountConfig {
	return s.MountSpec
}

// Validate the provisioning spec.
func (s VolumeDiscoverySpec) Validate(required bool) ([]string, error) {
	var validationErrors error

	if !s.VolumeSelectorConfig.Match.IsZero() {
		if err := s.VolumeSelectorConfig.Match.ParseBool(celenv.VolumeLocator()); err != nil {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("volume selector is invalid: %w", err))
		}
	} else {
		validationErrors = errors.Join(validationErrors, errors.New("volume selector is required"))
	}

	return nil, validationErrors
}

// VolumeSelector implements config.VolumeDiscoveryConfig interface.
func (s VolumeDiscoverySpec) VolumeSelector() cel.Expression {
	return s.VolumeSelectorConfig.Match
}

// ReadOnly implements config.VolumeMountConfig interface.
func (s MountSpec) ReadOnly() bool {
	return pointer.SafeDeref(s.MountReadOnly)
}
