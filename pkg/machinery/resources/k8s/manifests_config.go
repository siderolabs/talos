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

// BootstrapManifestsConfigType is type of BootstrapManifestsConfig resource.
const BootstrapManifestsConfigType = resource.Type("BootstrapManifestsConfigs.kubernetes.talos.dev")

// BootstrapManifestsConfigID is a singleton resource ID for BootstrapManifestsConfig.
const BootstrapManifestsConfigID = resource.ID("manifests")

// BootstrapManifestsConfig represents configuration for bootstrap manifests.
type BootstrapManifestsConfig = typed.Resource[BootstrapManifestsConfigSpec, BootstrapManifestsConfigRD]

// BootstrapManifestsConfigSpec is configuration for bootstrap manifests.
type BootstrapManifestsConfigSpec struct {
	Server        string `yaml:"string"`
	ClusterDomain string `yaml:"clusterDomain"`

	PodCIDRs []string `yaml:"podCIDRs"`

	ProxyEnabled bool     `yaml:"proxyEnabled"`
	ProxyImage   string   `yaml:"proxyImage"`
	ProxyArgs    []string `yaml:"proxyArgs"`

	CoreDNSEnabled bool   `yaml:"coreDNSEnabled"`
	CoreDNSImage   string `yaml:"coreDNSImage"`

	DNSServiceIP   string `yaml:"dnsServiceIP"`
	DNSServiceIPv6 string `yaml:"dnsServiceIPv6"`

	FlannelEnabled  bool   `yaml:"flannelEnabled"`
	FlannelImage    string `yaml:"flannelImage"`
	FlannelCNIImage string `yaml:"flannelCNIImage"`

	PodSecurityPolicyEnabled bool `yaml:"podSecurityPolicyEnabled"`

	TalosAPIServiceEnabled bool `yaml:"talosAPIServiceEnabled"`
}

// NewBootstrapManifestsConfig returns new BootstrapManifestsConfig resource.
func NewBootstrapManifestsConfig() *BootstrapManifestsConfig {
	return typed.NewResource[BootstrapManifestsConfigSpec, BootstrapManifestsConfigRD](
		resource.NewMetadata(ControlPlaneNamespaceName, BootstrapManifestsConfigType, BootstrapManifestsConfigID, resource.VersionUndefined),
		BootstrapManifestsConfigSpec{})
}

// BootstrapManifestsConfigRD defines BootstrapManifestsConfig resource definition.
type BootstrapManifestsConfigRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (BootstrapManifestsConfigRD) ResourceDefinition(_ resource.Metadata, _ BootstrapManifestsConfigSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BootstrapManifestsConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}
