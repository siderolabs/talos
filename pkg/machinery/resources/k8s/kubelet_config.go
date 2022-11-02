// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// KubeletConfigType is type of KubeletConfig resource.
const KubeletConfigType = resource.Type("KubeletConfigs.kubernetes.talos.dev")

// KubeletID is the ID of KubeletConfig resource.
const KubeletID = resource.ID("kubelet")

// KubeletConfig resource holds source of kubelet configuration.
type KubeletConfig = typed.Resource[KubeletConfigSpec, KubeletConfigRD]

// KubeletConfigSpec holds the source of kubelet configuration.
//
//gotagsrewrite:gen
type KubeletConfigSpec struct {
	Image                        string                 `yaml:"image" protobuf:"1"`
	ClusterDNS                   []string               `yaml:"clusterDNS" protobuf:"2"`
	ClusterDomain                string                 `yaml:"clusterDomain" protobuf:"3"`
	ExtraArgs                    map[string]string      `yaml:"extraArgs,omitempty" protobuf:"4"`
	ExtraMounts                  []specs.Mount          `yaml:"extraMounts,omitempty" protobuf:"5"`
	ExtraConfig                  map[string]interface{} `yaml:"extraConfig,omitempty" protobuf:"6"`
	CloudProviderExternal        bool                   `yaml:"cloudProviderExternal" protobuf:"7"`
	DefaultRuntimeSeccompEnabled bool                   `yaml:"defaultRuntimeSeccompEnabled" protobuf:"8"`
	SkipNodeRegistration         bool                   `yaml:"skipNodeRegistration" protobuf:"9"`
	StaticPodListURL             string                 `yaml:"staticPodListURL" protobuf:"10"`
	DisableManifestsDirectory    bool                   `yaml:"disableManifestsDirectory" protobuf:"11"`
}

// NewKubeletConfig initializes an empty KubeletConfig resource.
func NewKubeletConfig(namespace resource.Namespace, id resource.ID) *KubeletConfig {
	return typed.NewResource[KubeletConfigSpec, KubeletConfigRD](
		resource.NewMetadata(namespace, KubeletConfigType, id, resource.VersionUndefined),
		KubeletConfigSpec{},
	)
}

// KubeletConfigRD provides auxiliary methods for KubeletConfig.
type KubeletConfigRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (KubeletConfigRD) ResourceDefinition(resource.Metadata, KubeletConfigSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubeletConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KubeletConfigSpec](KubeletConfigType, &KubeletConfig{})
	if err != nil {
		panic(err)
	}
}
