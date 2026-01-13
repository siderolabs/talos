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

// SchedulerConfigType is type of SchedulerConfig resource.
const SchedulerConfigType = resource.Type("SchedulerConfigs.kubernetes.talos.dev")

// SchedulerConfigID is a singleton resource ID for SchedulerConfig.
const SchedulerConfigID = resource.ID(SchedulerID)

// SchedulerConfig represents configuration for kube-scheduler.
type SchedulerConfig = typed.Resource[SchedulerConfigSpec, SchedulerConfigExtension]

// SchedulerConfigSpec is configuration for kube-scheduler.
//
//gotagsrewrite:gen
type SchedulerConfigSpec struct {
	Enabled              bool                 `yaml:"enabled" protobuf:"1"`
	Image                string               `yaml:"image" protobuf:"2"`
	ExtraArgs            map[string]ArgValues `yaml:"extraArgs" protobuf:"3"`
	ExtraVolumes         []ExtraVolume        `yaml:"extraVolumes" protobuf:"4"`
	EnvironmentVariables map[string]string    `yaml:"environmentVariables" protobuf:"5"`
	Resources            Resources            `yaml:"resources" protobuf:"6"`
	Config               map[string]any       `yaml:"config" protobuf:"7"`
}

// NewSchedulerConfig returns new SchedulerConfig resource.
func NewSchedulerConfig() *SchedulerConfig {
	return typed.NewResource[SchedulerConfigSpec, SchedulerConfigExtension](
		resource.NewMetadata(ControlPlaneNamespaceName, SchedulerConfigType, SchedulerConfigID, resource.VersionUndefined),
		SchedulerConfigSpec{})
}

// SchedulerConfigExtension defines SchedulerConfig resource definition.
type SchedulerConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (SchedulerConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SchedulerConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[SchedulerConfigSpec](SchedulerConfigType, &SchedulerConfig{})
	if err != nil {
		panic(err)
	}
}
