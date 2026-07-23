// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"errors"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// KernelModuleConfigKind is a kernel module config document kind.
const KernelModuleConfigKind = "KernelModuleConfig"

func init() {
	registry.Register(KernelModuleConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KernelModuleConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.KernelModuleConfig = &KernelModuleConfigV1Alpha1{}
	_ config.NamedDocument      = &KernelModuleConfigV1Alpha1{}
	_ config.Validator          = &KernelModuleConfigV1Alpha1{}
)

// KernelModuleConfigV1Alpha1 is a config document to configure a Linux kernel module to load.
//
//	examples:
//	  - value: exampleKernelModuleConfigV1Alpha1()
//	alias: KernelModuleConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KernelModuleConfig
type KernelModuleConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Module name.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Module parameters, changes applied after reboot.
	ModuleParameters []string `yaml:"parameters,omitempty"`
}

// NewKernelModuleConfigV1Alpha1 creates a new KernelModuleConfig config document.
func NewKernelModuleConfigV1Alpha1(name string) *KernelModuleConfigV1Alpha1 {
	return &KernelModuleConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       KernelModuleConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleKernelModuleConfigV1Alpha1() *KernelModuleConfigV1Alpha1 {
	return NewKernelModuleConfigV1Alpha1("btrfs")
}

// Clone implements config.Document interface.
func (s *KernelModuleConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *KernelModuleConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Parameters implements config.KernelModuleConfig interface.
func (s *KernelModuleConfigV1Alpha1) Parameters() []string {
	return s.ModuleParameters
}

// Validate implements config.Validator interface.
func (s *KernelModuleConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if s.MetaName == "" {
		return nil, errors.New("name is required")
	}

	return nil, nil
}
