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

// APIServerConfigType is type of APIServerConfig resource.
const APIServerConfigType = resource.Type("APIServerConfigs.kubernetes.talos.dev")

// APIServerConfigID is a singleton resource ID for APIServerConfig.
const APIServerConfigID = resource.ID(APIServerID)

// APIServerConfig represents configuration for kube-apiserver.
type APIServerConfig = typed.Resource[APIServerConfigSpec, APIServerConfigExtension]

// ExtraVolume is a configuration of extra volume.
//
//gotagsrewrite:gen
type ExtraVolume struct {
	Name      string `yaml:"name" protobuf:"1"`
	HostPath  string `yaml:"hostPath" protobuf:"2"`
	MountPath string `yaml:"mountPath" protobuf:"3"`
	ReadOnly  bool   `yaml:"readonly" protobuf:"4"`
}

// Resources is a configuration of cpu and memory resources.
//
//gotagsrewrite:gen
type Resources struct {
	Requests map[string]string `yaml:"requests" protobuf:"1"`
	Limits   map[string]string `yaml:"limits" protobuf:"2"`
}

// APIServerConfigSpec is configuration for kube-apiserver.
//
//gotagsrewrite:gen
type APIServerConfigSpec struct {
	Image                string               `yaml:"image" protobuf:"1"`
	CloudProvider        string               `yaml:"cloudProvider" protobuf:"2"`
	ControlPlaneEndpoint string               `yaml:"controlPlaneEndpoint" protobuf:"3"`
	EtcdServers          []string             `yaml:"etcdServers" protobuf:"4"`
	LocalPort            int                  `yaml:"localPort" protobuf:"5"`
	ServiceCIDRs         []string             `yaml:"serviceCIDR" protobuf:"6"`
	ExtraArgs            map[string]ArgValues `yaml:"extraArgs" protobuf:"7"`
	ExtraVolumes         []ExtraVolume        `yaml:"extraVolumes" protobuf:"8"`
	EnvironmentVariables map[string]string    `yaml:"environmentVariables" protobuf:"9"`
	AdvertisedAddress    string               `yaml:"advertisedAddress" protobuf:"11"`
	Resources            Resources            `yaml:"resources" protobuf:"12"`
}

// ArgValues represents values for a command line argument which can be specified multiple times.
//
//gotagsrewrite:gen
type ArgValues struct {
	Values []string `yaml:"values" protobuf:"1"`
}

// NewAPIServerConfig returns new APIServerConfig resource.
func NewAPIServerConfig() *APIServerConfig {
	return typed.NewResource[APIServerConfigSpec, APIServerConfigExtension](
		resource.NewMetadata(ControlPlaneNamespaceName, APIServerConfigType, APIServerConfigID, resource.VersionUndefined),
		APIServerConfigSpec{})
}

// APIServerConfigExtension defines APIServerConfig resource definition.
type APIServerConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (APIServerConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             APIServerConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[APIServerConfigSpec](APIServerConfigType, &APIServerConfig{})
	if err != nil {
		panic(err)
	}
}
