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

// ControllerManagerConfigType is type of ControllerManagerConfig resource.
const ControllerManagerConfigType = resource.Type("ControllerManagerConfigs.kubernetes.talos.dev")

// ControllerManagerConfigID is a singleton resource ID for ControllerManagerConfig.
const ControllerManagerConfigID = resource.ID(ControllerManagerID)

// ControllerManagerConfig represents configuration for kube-controller-manager.
type ControllerManagerConfig = typed.Resource[ControllerManagerConfigSpec, ControllerManagerConfigRD]

// ControllerManagerConfigSpec is configuration for kube-controller-manager.
type ControllerManagerConfigSpec struct {
	Enabled              bool              `yaml:"enabled"`
	Image                string            `yaml:"image"`
	CloudProvider        string            `yaml:"cloudProvider"`
	PodCIDRs             []string          `yaml:"podCIDRs"`
	ServiceCIDRs         []string          `yaml:"serviceCIDRs"`
	ExtraArgs            map[string]string `yaml:"extraArgs"`
	ExtraVolumes         []ExtraVolume     `yaml:"extraVolumes"`
	EnvironmentVariables map[string]string `yaml:"environmentVariables"`
}

// NewControllerManagerConfig returns new ControllerManagerConfig resource.
func NewControllerManagerConfig() *ControllerManagerConfig {
	return typed.NewResource[ControllerManagerConfigSpec, ControllerManagerConfigRD](
		resource.NewMetadata(ControlPlaneNamespaceName, ControllerManagerConfigType, ControllerManagerConfigID, resource.VersionUndefined),
		ControllerManagerConfigSpec{})
}

// ControllerManagerConfigRD defines ControllerManagerConfig resource definition.
type ControllerManagerConfigRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ControllerManagerConfigRD) ResourceDefinition(_ resource.Metadata, _ ControllerManagerConfigSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ControllerManagerConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}
