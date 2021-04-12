// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/encoder"
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

func (s *v1alpha1Spec) MarshalYAML() (interface{}, error) {
	return encoder.NewEncoder(s.cfg).Marshal()
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
	b, err := r.spec.cfg.Bytes()
	if err != nil {
		panic(err) // TODO: DeepCopy() should support returning errors? or config should implement DeeCopy without errors?
	}

	c, err := configloader.NewFromBytes(b)
	if err != nil {
		panic(err)
	}

	return &MachineConfig{
		md: r.md,
		spec: &v1alpha1Spec{
			cfg: c.(*v1alpha1.Config),
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *MachineConfig) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

// Config returns config.Provider.
func (r *MachineConfig) Config() config.Provider {
	return r.spec.cfg
}
