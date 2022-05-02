// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// KubeletSpecType is type of KubeletSpec resource.
const KubeletSpecType = resource.Type("KubeletSpecs.kubernetes.talos.dev")

// KubeletSpec resource holds final definition of kubelet runtime configuration.
type KubeletSpec = typed.Resource[KubeletSpecSpec, KubeletSpecRD]

// KubeletSpecSpec holds the source of kubelet configuration.
type KubeletSpecSpec struct {
	Image       string                 `yaml:"image"`
	Args        []string               `yaml:"args,omitempty"`
	ExtraMounts []specs.Mount          `yaml:"extraMounts,omitempty"`
	Config      map[string]interface{} `yaml:"config"`
}

// DeepCopy implements typed.DeepCopyable interface.
func (spec KubeletSpecSpec) DeepCopy() KubeletSpecSpec {
	config := make(map[string]interface{}, len(spec.Config))

	for k, v := range spec.Config {
		config[k] = v
	}

	return KubeletSpecSpec{
		Image:       spec.Image,
		Args:        append([]string(nil), spec.Args...),
		ExtraMounts: append([]specs.Mount(nil), spec.ExtraMounts...),
		Config:      config,
	}
}

// NewKubeletSpec initializes an empty KubeletSpec resource.
func NewKubeletSpec(namespace resource.Namespace, id resource.ID) *KubeletSpec {
	return typed.NewResource[KubeletSpecSpec, KubeletSpecRD](
		resource.NewMetadata(namespace, KubeletSpecType, id, resource.VersionUndefined),
		KubeletSpecSpec{},
	)
}

// KubeletSpecRD provides auxiliary methods for KubeletSpec.
type KubeletSpecRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (KubeletSpecRD) ResourceDefinition(resource.Metadata, KubeletSpecSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubeletSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}
