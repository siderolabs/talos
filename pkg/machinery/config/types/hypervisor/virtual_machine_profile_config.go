// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// VirtualMachineProfileConfigKind identifies the named guest hardware document.
const VirtualMachineProfileConfigKind = "VirtualMachineProfileConfig"

func init() {
	registry.Register(VirtualMachineProfileConfigKind, func(version string) config.Document {
		if version == versionV1Alpha1 {
			return &VirtualMachineProfileConfigV1Alpha1{}
		}

		return nil
	})
}

var (
	_ config.VirtualMachineProfileConfig           = &VirtualMachineProfileConfigV1Alpha1{}
	_ config.VirtualMachineProfileCPUConfig        = &VirtualMachineProfileCPU{}
	_ config.VirtualMachineProfileCPUFeatureConfig = &VirtualMachineProfileCPUFeature{}
	_ config.Validator                             = &VirtualMachineProfileConfigV1Alpha1{}
)

// VirtualMachineProfileConfigV1Alpha1 declares a named amd64 guest ABI.
//
//	description: |
//	  Profiles bundle the guest machine type and CPU model. Use a versioned machine
//	  type and a CPU baseline tested on every host for migration. Aliases drift as QEMU
//	  changes. A named CPU model is not a migration guarantee across arbitrary hosts.
//	examples:
//	  - value: exampleVirtualMachineProfileConfigV1Alpha1()
//	alias: VirtualMachineProfileConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/VirtualMachineProfileConfig
type VirtualMachineProfileConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	// description: Profile name. Builtin names and the compat- prefix are reserved.
	// schemaRequired: true
	MetaName string `yaml:"name"`
	// description: q35 or i440fx machine family, preferably a versioned type.
	// schemaRequired: true
	MachineConfig string `yaml:"machine"`
	// description: Guest CPU model and feature policy.
	// schemaRequired: true
	CPUConfig VirtualMachineProfileCPU `yaml:"cpu"`
}

// VirtualMachineProfileCPU selects the guest-visible CPU.
type VirtualMachineProfileCPU struct {
	//   description: host-model, host-passthrough or custom.
	//   values:
	//     - host-model
	//     - host-passthrough
	//     - custom
	//   schemaRequired: true
	ModeConfig hypervisorhelpers.VirtualMachineProfileCPUMode `yaml:"mode"`
	// description: Named CPU model; required only for custom.
	ModelConfig string `yaml:"model,omitempty"`
	// description: allow or forbid (default forbid) for custom model fallback.
	FallbackConfig string `yaml:"fallback,omitempty"`
	// description: Explicit feature policies for a custom model.
	FeaturesConfig []VirtualMachineProfileCPUFeature `yaml:"features,omitempty"`
}

// VirtualMachineProfileCPUFeature selects one CPU flag.
type VirtualMachineProfileCPUFeature struct {
	// description: CPU feature name.
	// schemaRequired: true
	NameConfig string `yaml:"name"`
	//   description: require, disable, optional, force or forbid.
	//   values:
	//     - require
	//     - disable
	//     - optional
	//     - force
	//     - forbid
	//   schemaRequired: true
	PolicyConfig hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicy `yaml:"policy"`
}

// NewVirtualMachineProfileConfigV1Alpha1 creates a named profile document.
func NewVirtualMachineProfileConfigV1Alpha1() *VirtualMachineProfileConfigV1Alpha1 {
	return &VirtualMachineProfileConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       VirtualMachineProfileConfigKind,
			MetaAPIVersion: versionV1Alpha1,
		},
	}
}

func exampleVirtualMachineProfileConfigV1Alpha1() *VirtualMachineProfileConfigV1Alpha1 {
	cfg := NewVirtualMachineProfileConfigV1Alpha1()
	cfg.MetaName = "rack-a-compat"
	cfg.MachineConfig = "pc-q35-9.2"
	cfg.CPUConfig = VirtualMachineProfileCPU{
		ModeConfig:  hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
		ModelConfig: "Haswell-noTSX-IBRS",
		FeaturesConfig: []VirtualMachineProfileCPUFeature{
			{
				NameConfig:   "pku",
				PolicyConfig: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyDisable,
			},
		},
	}

	return cfg
}

// Name implements config.NamedDocument.
func (c *VirtualMachineProfileConfigV1Alpha1) Name() string {
	return c.MetaName
}

// Clone implements config.Document.
func (c *VirtualMachineProfileConfigV1Alpha1) Clone() config.Document {
	return c.DeepCopy()
}

// VirtualMachineProfileConfigSignal identifies profile documents.
func (c *VirtualMachineProfileConfigV1Alpha1) VirtualMachineProfileConfigSignal() {}

// Machine implements config.VirtualMachineProfileConfig.
func (c *VirtualMachineProfileConfigV1Alpha1) Machine() string {
	return c.MachineConfig
}

// CPU implements config.VirtualMachineProfileConfig.
func (c *VirtualMachineProfileConfigV1Alpha1) CPU() config.VirtualMachineProfileCPUConfig {
	return &c.CPUConfig
}

// Mode implements config.VirtualMachineProfileCPUConfig.
func (c *VirtualMachineProfileCPU) Mode() hypervisorhelpers.VirtualMachineProfileCPUMode {
	return c.ModeConfig
}

// Model implements config.VirtualMachineProfileCPUConfig.
func (c *VirtualMachineProfileCPU) Model() string {
	return c.ModelConfig
}

// Fallback implements config.VirtualMachineProfileCPUConfig.
func (c *VirtualMachineProfileCPU) Fallback() string {
	if c.FallbackConfig == "" && c.ModeConfig == hypervisorhelpers.VirtualMachineProfileCPUModeCustom {
		return "forbid"
	}

	return c.FallbackConfig
}

// Features implements config.VirtualMachineProfileCPUConfig.
func (c *VirtualMachineProfileCPU) Features() []config.VirtualMachineProfileCPUFeatureConfig {
	out := make([]config.VirtualMachineProfileCPUFeatureConfig, 0, len(c.FeaturesConfig))
	for i := range c.FeaturesConfig {
		out = append(out, &c.FeaturesConfig[i])
	}

	return out
}

// Name implements config.VirtualMachineProfileCPUFeatureConfig.
func (c *VirtualMachineProfileCPUFeature) Name() string {
	return c.NameConfig
}

// Policy implements config.VirtualMachineProfileCPUFeatureConfig.
func (c *VirtualMachineProfileCPUFeature) Policy() hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicy {
	return c.PolicyConfig
}

// Validate checks structural constraints, never host capabilities.
func (c *VirtualMachineProfileConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	errs = errors.Join(errs, hypervisorhelpers.ValidateName(c.MetaName))
	if c.MetaName == "host" || c.MetaName == "legacy" || c.MetaName == "passthrough" || strings.HasPrefix(c.MetaName, "compat-") {
		errs = errors.Join(errs, fmt.Errorf("profile name %q is reserved", c.MetaName))
	}

	if !hypervisorhelpers.ValidVirtualMachineProfileMachine(c.MachineConfig) {
		errs = errors.Join(errs, fmt.Errorf("machine %q must be q35, pc, pc-q35-* or pc-i440fx-*", c.MachineConfig))
	}

	if err := hypervisorhelpers.ValidateVirtualMachineProfileCPU(c.CPUConfig.ModeConfig, c.CPUConfig.ModelConfig, c.CPUConfig.FallbackConfig, func() []hypervisorhelpers.VirtualMachineProfileFeature {
		features := make([]hypervisorhelpers.VirtualMachineProfileFeature, 0, len(c.CPUConfig.FeaturesConfig))
		for _, f := range c.CPUConfig.FeaturesConfig {
			features = append(features, hypervisorhelpers.VirtualMachineProfileFeature{Name: f.NameConfig, Policy: f.PolicyConfig})
		}

		return features
	}()); err != nil {
		errs = errors.Join(errs, err)
	}

	return nil, errs
}
