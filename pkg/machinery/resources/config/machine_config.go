// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"

	configpb "github.com/siderolabs/talos/pkg/machinery/api/resource/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// MachineConfigType is type of Service resource.
const MachineConfigType = resource.Type("MachineConfigs.config.talos.dev")

// V1Alpha1ID is the ID of V1Alpha1 resource (singleton).
const V1Alpha1ID = resource.ID("v1alpha1")

// MaintenanceID is the ID of the config submitted in the maintenance mode.
const MaintenanceID = resource.ID("maintenance")

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
	return NewMachineConfigWithID(spec, V1Alpha1ID)
}

// NewMachineConfigWithID initializes a MachineConfig resource.
func NewMachineConfigWithID(spec config.Provider, id resource.ID) *MachineConfig {
	r := &MachineConfig{
		md: resource.NewMetadata(NamespaceName, MachineConfigType, id, resource.VersionUndefined),
		spec: &v1alpha1Spec{
			cfg: spec,
		},
	}

	return r
}

// Metadata implements resource.Resource.
func (r *MachineConfig) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *MachineConfig) Spec() any {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *MachineConfig) DeepCopy() resource.Resource {
	cfgCopy := r.spec.cfg

	if !cfgCopy.Readonly() {
		cfgCopy = cfgCopy.Clone()
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

// MarshalProto implements ProtoMarshaler.
func (s *v1alpha1Spec) MarshalProto() ([]byte, error) {
	yamlBytes, err := s.cfg.Bytes()
	if err != nil {
		return nil, err
	}

	protoSpec := configpb.MachineConfigSpec{
		YamlMarshalled: yamlBytes,
	}

	return proto.Marshal(&protoSpec)
}

// UnmarshalProto implements protobuf.ResourceUnmarshaler.
func (r *MachineConfig) UnmarshalProto(md *resource.Metadata, protoBytes []byte) error {
	protoSpec := configpb.MachineConfigSpec{}

	if err := proto.Unmarshal(protoBytes, &protoSpec); err != nil {
		return err
	}

	cfg, err := configloader.NewFromBytes(protoSpec.YamlMarshalled)
	if err != nil {
		return err
	}

	r.md = *md
	r.spec = &v1alpha1Spec{
		cfg: cfg,
	}

	return nil
}

// Config returns config.Config to access config fields.
func (r *MachineConfig) Config() config.Config {
	return r.spec.cfg
}

// Container returns config.Container to access config as a whole.
func (r *MachineConfig) Container() config.Container {
	return r.spec.cfg
}

// Provider returns config.Provider to access config provider.
//
// This method should only be used when Container() and Config() methods are not enough.
// Config()/Container() provides better semantic split on the config access.
func (r *MachineConfig) Provider() config.Provider {
	return r.spec.cfg
}

func init() {
	if err := protobuf.RegisterResource(MachineConfigType, &MachineConfig{}); err != nil {
		panic(err)
	}
}
