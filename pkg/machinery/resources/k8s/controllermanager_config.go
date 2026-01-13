// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides resources which interface with Kubernetes.
package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ControllerManagerConfigType is type of ControllerManagerConfig resource.
const ControllerManagerConfigType = resource.Type("ControllerManagerConfigs.kubernetes.talos.dev")

// ControllerManagerConfigID is a singleton resource ID for ControllerManagerConfig.
const ControllerManagerConfigID = resource.ID(ControllerManagerID)

// ControllerManagerConfig represents configuration for kube-controller-manager.
type ControllerManagerConfig = typed.Resource[ControllerManagerConfigSpec, ControllerManagerConfigExtension]

// ControllerManagerConfigSpec is configuration for kube-controller-manager.
//
//gotagsrewrite:gen
type ControllerManagerConfigSpec struct {
	Enabled              bool                 `yaml:"enabled" protobuf:"1"`
	Image                string               `yaml:"image" protobuf:"2"`
	CloudProvider        string               `yaml:"cloudProvider" protobuf:"3"`
	PodCIDRs             []string             `yaml:"podCIDRs" protobuf:"4"`
	ServiceCIDRs         []string             `yaml:"serviceCIDRs" protobuf:"5"`
	ExtraArgs            map[string]ArgValues `yaml:"extraArgs" protobuf:"6"`
	ExtraVolumes         []ExtraVolume        `yaml:"extraVolumes" protobuf:"7"`
	EnvironmentVariables map[string]string    `yaml:"environmentVariables" protobuf:"8"`
	Resources            Resources            `yaml:"resources" protobuf:"9"`
}

// NewControllerManagerConfig returns new ControllerManagerConfig resource.
func NewControllerManagerConfig() *ControllerManagerConfig {
	return typed.NewResource[ControllerManagerConfigSpec, ControllerManagerConfigExtension](
		resource.NewMetadata(ControlPlaneNamespaceName, ControllerManagerConfigType, ControllerManagerConfigID, resource.VersionUndefined),
		ControllerManagerConfigSpec{})
}

// ControllerManagerConfigExtension defines ControllerManagerConfig resource definition.
type ControllerManagerConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ControllerManagerConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ControllerManagerConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ControllerManagerConfigSpec](ControllerManagerConfigType, &ControllerManagerConfig{})
	if err != nil {
		panic(err)
	}
}
