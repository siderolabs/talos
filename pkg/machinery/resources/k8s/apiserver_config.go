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

// APIServerConfigType is type of APIServerConfig resource.
const APIServerConfigType = resource.Type("APIServerConfigs.kubernetes.talos.dev")

// APIServerConfigID is a singleton resource ID for APIServerConfig.
const APIServerConfigID = resource.ID(APIServerID)

// APIServerConfig represents configuration for kube-apiserver.
type APIServerConfig = typed.Resource[APIServerConfigSpec, APIServerConfigRD]

// ExtraVolume is a configuration of extra volume.
type ExtraVolume struct {
	Name      string `yaml:"name"`
	HostPath  string `yaml:"hostPath"`
	MountPath string `yaml:"mountPath"`
	ReadOnly  bool   `yaml:"readonly"`
}

// APIServerConfigSpec is configuration for kube-apiserver.
type APIServerConfigSpec struct {
	Image                    string            `yaml:"image"`
	CloudProvider            string            `yaml:"cloudProvider"`
	ControlPlaneEndpoint     string            `yaml:"controlPlaneEndpoint"`
	EtcdServers              []string          `yaml:"etcdServers"`
	LocalPort                int               `yaml:"localPort"`
	ServiceCIDRs             []string          `yaml:"serviceCIDR"`
	ExtraArgs                map[string]string `yaml:"extraArgs"`
	ExtraVolumes             []ExtraVolume     `yaml:"extraVolumes"`
	EnvironmentVariables     map[string]string `yaml:"environmentVariables"`
	PodSecurityPolicyEnabled bool              `yaml:"podSecurityPolicyEnabled"`
}

// DeepCopy implements Deepcopyable.
//
// TODO: should be properly go-generated.
func (spec APIServerConfigSpec) DeepCopy() APIServerConfigSpec {
	return spec
}

// NewAPIServerConfig returns new APIServerConfig resource.
func NewAPIServerConfig() *APIServerConfig {
	return typed.NewResource[APIServerConfigSpec, APIServerConfigRD](
		resource.NewMetadata(ControlPlaneNamespaceName, APIServerConfigType, APIServerConfigID, resource.VersionUndefined),
		APIServerConfigSpec{})
}

// APIServerConfigRD defines APIServerConfig resource definition.
type APIServerConfigRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (APIServerConfigRD) ResourceDefinition(_ resource.Metadata, _ APIServerConfigSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             APIServerConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}
