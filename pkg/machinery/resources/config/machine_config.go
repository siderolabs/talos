// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

// MachineConfigType is type of Service resource.
const MachineConfigType = resource.Type("MachineConfigs.config.talos.dev")

// V1Alpha1ID is the ID of V1Alpha1 resource (singleton).
const V1Alpha1ID = resource.ID("v1alpha1")

// MachineConfig resource holds v1alpha Talos configuration.
type MachineConfig struct {
	md   resource.Metadata
	spec *v1alpha1Spec
}

type v1alpha1Spec struct {
	cfg config.Provider
}

// MarshalYAMLBytes implements RawYAML interface.
func (s *v1alpha1Spec) MarshalYAMLBytes() ([]byte, error) {
	return s.cfg.Bytes()
}

// NewMachineConfig initializes a V1Alpha1 resource.
func NewMachineConfig(spec config.Provider) *MachineConfig {
	r := &MachineConfig{
		md: resource.NewMetadata(NamespaceName, MachineConfigType, V1Alpha1ID, resource.VersionUndefined),
		spec: &v1alpha1Spec{
			cfg: spec,
		},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *MachineConfig) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *MachineConfig) Spec() interface{} {
	return r.spec
}

func (r *MachineConfig) String() string {
	return fmt.Sprintf("config.MachineConfig(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *MachineConfig) DeepCopy() resource.Resource {
	var cfgCopy config.Provider

	switch r.spec.cfg.(type) {
	case *v1alpha1.ReadonlyProvider:
		// don't copy read only config
		cfgCopy = r.spec.cfg
	default:
		cfgCopy = r.spec.cfg.Raw().(*v1alpha1.Config).DeepCopy()
	}

	return &MachineConfig{
		md: r.md,
		spec: &v1alpha1Spec{
			cfg: cfgCopy,
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *MachineConfig) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

// Config returns config.Provider.
func (r *MachineConfig) Config() config.Provider {
	return r.spec.cfg
}
