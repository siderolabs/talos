// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

// KubeletConfigType is type of KubeletConfig resource.
const KubeletConfigType = resource.Type("KubeletConfigs.kubernetes.talos.dev")

// KubeletID is the ID of KubeletConfig resource.
const KubeletID = resource.ID("kubelet")

// KubeletConfig resource holds source of kubelet configuration.
type KubeletConfig struct {
	md   resource.Metadata
	spec *KubeletConfigSpec
}

// KubeletConfigSpec holds the source of kubelet configuration.
type KubeletConfigSpec struct {
	Image                 string                 `yaml:"image"`
	ClusterDNS            []string               `yaml:"clusterDNS"`
	ClusterDomain         string                 `yaml:"clusterDomain"`
	ExtraArgs             map[string]string      `yaml:"extraArgs,omitempty"`
	ExtraMounts           []specs.Mount          `yaml:"extraMounts,omitempty"`
	ExtraConfig           map[string]interface{} `yaml:"extraConfig,omitempty"`
	CloudProviderExternal bool                   `yaml:"cloudProviderExternal"`
}

// NewKubeletConfig initializes an empty KubeletConfig resource.
func NewKubeletConfig(namespace resource.Namespace, id resource.ID) *KubeletConfig {
	r := &KubeletConfig{
		md:   resource.NewMetadata(namespace, KubeletConfigType, id, resource.VersionUndefined),
		spec: &KubeletConfigSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *KubeletConfig) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *KubeletConfig) Spec() interface{} {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *KubeletConfig) DeepCopy() resource.Resource {
	extraArgs := make(map[string]string, len(r.spec.ExtraArgs))

	for k, v := range r.spec.ExtraArgs {
		extraArgs[k] = v
	}

	extraConfig := &v1alpha1.Unstructured{Object: r.spec.ExtraConfig}
	extraConfig = extraConfig.DeepCopy()

	return &KubeletConfig{
		md: r.md,
		spec: &KubeletConfigSpec{
			Image:                 r.spec.Image,
			ClusterDNS:            append([]string(nil), r.spec.ClusterDNS...),
			ClusterDomain:         r.spec.ClusterDomain,
			ExtraArgs:             extraArgs,
			ExtraMounts:           append([]specs.Mount(nil), r.spec.ExtraMounts...),
			ExtraConfig:           extraConfig.Object,
			CloudProviderExternal: r.spec.CloudProviderExternal,
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *KubeletConfig) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubeletConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

// TypedSpec returns .spec.
func (r *KubeletConfig) TypedSpec() *KubeletConfigSpec {
	return r.spec
}
