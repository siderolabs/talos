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

// BootstrapManifestsConfigType is type of BootstrapManifestsConfig resource.
const BootstrapManifestsConfigType = resource.Type("BootstrapManifestsConfigs.kubernetes.talos.dev")

// BootstrapManifestsConfigID is a singleton resource ID for BootstrapManifestsConfig.
const BootstrapManifestsConfigID = resource.ID("manifests")

// BootstrapManifestsConfig represents configuration for bootstrap manifests.
type BootstrapManifestsConfig = typed.Resource[BootstrapManifestsConfigSpec, BootstrapManifestsConfigExtension]

// BootstrapManifestsConfigSpec is configuration for bootstrap manifests.
//
//gotagsrewrite:gen
type BootstrapManifestsConfigSpec struct {
	Server        string `yaml:"string" protobuf:"1"`
	ClusterDomain string `yaml:"clusterDomain" protobuf:"2"`

	PodCIDRs []string `yaml:"podCIDRs" protobuf:"3"`

	ProxyEnabled bool     `yaml:"proxyEnabled" protobuf:"4"`
	ProxyImage   string   `yaml:"proxyImage" protobuf:"5"`
	ProxyArgs    []string `yaml:"proxyArgs" protobuf:"6"`

	CoreDNSEnabled bool   `yaml:"coreDNSEnabled" protobuf:"7"`
	CoreDNSImage   string `yaml:"coreDNSImage" protobuf:"8"`

	DNSServiceIP   string `yaml:"dnsServiceIP" protobuf:"9"`
	DNSServiceIPv6 string `yaml:"dnsServiceIPv6" protobuf:"10"`

	FlannelEnabled         bool     `yaml:"flannelEnabled" protobuf:"11"`
	FlannelImage           string   `yaml:"flannelImage" protobuf:"12"`
	FlannelExtraArgs       []string `yaml:"flannelExtraArgs" protobuf:"16"`
	FlannelKubeServiceHost string   `yaml:"flannelKubeServiceHost" protobuf:"17"`
	FlannelKubeServicePort string   `yaml:"flannelKubeServicePort" protobuf:"18"`

	PodSecurityPolicyEnabled bool `yaml:"podSecurityPolicyEnabled" protobuf:"14"`

	TalosAPIServiceEnabled bool `yaml:"talosAPIServiceEnabled" protobuf:"15"`
}

// NewBootstrapManifestsConfig returns new BootstrapManifestsConfig resource.
func NewBootstrapManifestsConfig() *BootstrapManifestsConfig {
	return typed.NewResource[BootstrapManifestsConfigSpec, BootstrapManifestsConfigExtension](
		resource.NewMetadata(ControlPlaneNamespaceName, BootstrapManifestsConfigType, BootstrapManifestsConfigID, resource.VersionUndefined),
		BootstrapManifestsConfigSpec{})
}

// BootstrapManifestsConfigExtension defines BootstrapManifestsConfig resource definition.
type BootstrapManifestsConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (BootstrapManifestsConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BootstrapManifestsConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[BootstrapManifestsConfigSpec](BootstrapManifestsConfigType, &BootstrapManifestsConfig{})
	if err != nil {
		panic(err)
	}
}
