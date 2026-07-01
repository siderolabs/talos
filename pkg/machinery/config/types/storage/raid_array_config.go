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
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// RAIDArrayConfigKind is a config document kind.
const RAIDArrayConfigKind = "RAIDArrayConfig"

func init() {
	registry.Register(RAIDArrayConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &RAIDArrayConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.RAIDArrayConfig = &RAIDArrayConfigV1Alpha1{}
	_ config.NamedDocument   = &RAIDArrayConfigV1Alpha1{}
	_ config.Validator       = &RAIDArrayConfigV1Alpha1{}
)

// mdadm stamps the array name into the metadata; keep it short for a stable
// by-id device path and resource ID.
const maxRAIDArrayNameLength = 63

// RAIDArrayConfigV1Alpha1 provisions a Linux MD (software RAID) array.
//
//	description: |
//	  Provisions a Linux software RAID (MD) array from matching disks.
//
//	  The array is exposed at `/dev/disk/by-id/md-name-<name>` and can back a
//	  user volume. Provisioning is additive: the array and its members are
//	  created but never destroyed by this document. Use `talosctl wipe md` to
//	  remove an array.
//	examples:
//	  - value: exampleRAIDArrayConfigV1Alpha1()
//	alias: RAIDArrayConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/RAIDArrayConfig
type RAIDArrayConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Array name, stamped into the md metadata.
	//
	//     Must be 1-63 chars: ASCII letters, digits, hyphens, underscores.
	//     Exposed as `/dev/disk/by-id/md-name-<name>`.
	MetaName string `yaml:"name"`
	//   description: |
	//     RAID level.
	//   values:
	//     - raid1
	Level storageres.MDLevel `yaml:"level"`
	//   description: |
	//     The provisioning describes how the RAID arrays are provisioned.
	ProvisioningSpec RAIDProvisioningSpec `yaml:"provisioning"`
}

// RAIDProvisioningSpec describes how the RAID arrays are provisioned.
type RAIDProvisioningSpec struct {
	//   description: |
	//     The volume selector describes how the members of RAID arrays are selected.
	RAIDVolumeSelector RAIDVolumeSelector `yaml:"volumeSelector"`
}

// VolumeSelector returns the volume selector expression.
func (s RAIDProvisioningSpec) VolumeSelector() cel.Expression {
	return s.RAIDVolumeSelector.Match
}

// Validate parses selector without mutating stored config.
func (s RAIDProvisioningSpec) Validate() error {
	if s.RAIDVolumeSelector.Match.IsZero() {
		return errors.New("provisioning.volumeSelector.match is required")
	}

	if err := s.RAIDVolumeSelector.Match.ParseBool(celenv.DiskLocator()); err != nil {
		return fmt.Errorf("provisioning.volumeSelector.match: %w", err)
	}

	return nil
}

// RAIDVolumeSelector matches member disks with CEL.
type RAIDVolumeSelector struct {
	//   description: |
	//     CEL expression matching the member disks of the array.
	//
	//     Evaluated against each discovered disk with the `disk` variable.
	//   schema:
	//     type: string
	//   examples:
	//     - value: >
	//        exampleRAIDDiskSelector()
	//       name: match NVMe disks larger than 100 GiB
	Match cel.Expression `yaml:"match,omitempty"`
}

// IsZero reports whether the selector is empty.
func (s RAIDVolumeSelector) IsZero() bool {
	return s.Match.IsZero()
}

// NewRAIDArrayConfigV1Alpha1 creates a new RAIDArrayConfig document.
func NewRAIDArrayConfigV1Alpha1() *RAIDArrayConfigV1Alpha1 {
	return &RAIDArrayConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       RAIDArrayConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleRAIDArrayConfigV1Alpha1() *RAIDArrayConfigV1Alpha1 {
	cfg := NewRAIDArrayConfigV1Alpha1()
	cfg.MetaName = "talos"
	cfg.Level = storageres.MDLevelRAID1

	return cfg
}

func exampleRAIDDiskSelector() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "nvme" && disk.size > 100u * GiB`, celenv.DiskLocator()))
}

// Name implements config.NamedDocument interface.
func (s *RAIDArrayConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *RAIDArrayConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// RAIDArrayConfigSignal is a marker for config.RAIDArrayConfig.
func (s *RAIDArrayConfigV1Alpha1) RAIDArrayConfigSignal() {}

// RAIDLevel implements config.RAIDArrayConfig.
func (s *RAIDArrayConfigV1Alpha1) RAIDLevel() storageres.MDLevel {
	return s.Level
}

// Provisioning implements config.RAIDArrayConfig.
func (s *RAIDArrayConfigV1Alpha1) Provisioning() config.RAIDProvisioningConfig {
	return s.ProvisioningSpec
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo
func (s *RAIDArrayConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	if s.MetaName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	}

	if len(s.MetaName) < 1 || len(s.MetaName) > maxRAIDArrayNameLength {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("name must be between 1 and %d characters long", maxRAIDArrayNameLength))
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

	// Level is a typed enum; unsupported strings are rejected at decode time.

	if err := s.ProvisioningSpec.Validate(); err != nil {
		validationErrors = errors.Join(validationErrors, err)
	}

	return nil, validationErrors
}
