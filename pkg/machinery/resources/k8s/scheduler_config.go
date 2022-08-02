// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides resources which interface with Kubernetes.
//
//nolint:dupl
package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// SchedulerConfigType is type of SchedulerConfig resource.
const SchedulerConfigType = resource.Type("SchedulerConfigs.kubernetes.talos.dev")

// SchedulerConfigID is a singleton resource ID for SchedulerConfig.
const SchedulerConfigID = resource.ID(SchedulerID)

// SchedulerConfig represents configuration for kube-scheduler.
type SchedulerConfig = typed.Resource[SchedulerConfigSpec, SchedulerConfigRD]

// SchedulerConfigSpec is configuration for kube-scheduler.
//
//gotagsrewrite:gen
type SchedulerConfigSpec struct {
	Enabled              bool              `yaml:"enabled" protobuf:"1"`
	Image                string            `yaml:"image" protobuf:"2"`
	ExtraArgs            map[string]string `yaml:"extraArgs" protobuf:"3"`
	ExtraVolumes         []ExtraVolume     `yaml:"extraVolumes" protobuf:"4"`
	EnvironmentVariables map[string]string `yaml:"environmentVariables" protobuf:"5"`
}

// NewSchedulerConfig returns new SchedulerConfig resource.
func NewSchedulerConfig() *SchedulerConfig {
	return typed.NewResource[SchedulerConfigSpec, SchedulerConfigRD](
		resource.NewMetadata(ControlPlaneNamespaceName, SchedulerConfigType, SchedulerConfigID, resource.VersionUndefined),
		SchedulerConfigSpec{})
}

// SchedulerConfigRD defines SchedulerConfig resource definition.
type SchedulerConfigRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (SchedulerConfigRD) ResourceDefinition(_ resource.Metadata, _ SchedulerConfigSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SchedulerConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}
