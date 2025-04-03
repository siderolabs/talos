// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// UserVolumeConfigKind is a config document kind.
const UserVolumeConfigKind = "UserVolumeConfig"

func init() {
	registry.Register(UserVolumeConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &UserVolumeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.UserVolumeConfig = &UserVolumeConfigV1Alpha1{}
	_ config.NamedDocument    = &UserVolumeConfigV1Alpha1{}
	_ config.Validator        = &UserVolumeConfigV1Alpha1{}
)

// UserVolumeConfigV1Alpha1 is a user volume configuration document.
//
//	examples:
//	  - value: exampleUserVolumeConfigV1Alpha1()
//	alias: UserVolumeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/UserVolumeConfig
type UserVolumeConfigV1Alpha1 struct {
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
}

// NewUserVolumeConfigV1Alpha1 creates a new user volume config document.
func NewUserVolumeConfigV1Alpha1() *UserVolumeConfigV1Alpha1 {
	return &UserVolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       UserVolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleUserVolumeConfigV1Alpha1() *UserVolumeConfigV1Alpha1 {
	cfg := NewUserVolumeConfigV1Alpha1()
	cfg.MetaName = "ceph-data"
	cfg.ProvisioningSpec = ProvisioningSpec{
		DiskSelectorSpec: DiskSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`disk.transport == "nvme"`, celenv.DiskLocator())),
		},
		ProvisioningMaxSize: MustByteSize("50GiB"),
	}

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *UserVolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *UserVolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *UserVolumeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	if len(s.MetaName) < 1 || len(s.MetaName) > 34 {
		validationErrors = errors.Join(validationErrors, errors.New("name must be between 1 and 34 characters long"))
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
		default: //invalid symbol
			return true
		}
	}) {
		validationErrors = errors.Join(validationErrors, errors.New("name can only contain lowercase and uppercase ASCII letters, digits, and hyphens"))
	}

	return nil, validationErrors
}

// Provisioning implements config.UserVolumeConfig interface.
func (s *UserVolumeConfigV1Alpha1) Provisioning() config.VolumeProvisioningConfig {
	return s.ProvisioningSpec
}
