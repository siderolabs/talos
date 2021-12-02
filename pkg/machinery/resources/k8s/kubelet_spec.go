// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// KubeletSpecType is type of KubeletSpec resource.
const KubeletSpecType = resource.Type("KubeletSpecs.kubernetes.talos.dev")

// KubeletSpec resource holds final definition of kubelet runtime configuration.
type KubeletSpec struct {
	md   resource.Metadata
	spec *KubeletSpecSpec
}

// KubeletSpecSpec holds the source of kubelet configuration.
type KubeletSpecSpec struct {
	Image       string                 `yaml:"image"`
	Args        []string               `yaml:"args,omitempty"`
	ExtraMounts []specs.Mount          `yaml:"extraMounts,omitempty"`
	Config      map[string]interface{} `yaml:"config"`
}

// NewKubeletSpec initializes an empty KubeletSpec resource.
func NewKubeletSpec(namespace resource.Namespace, id resource.ID) *KubeletSpec {
	r := &KubeletSpec{
		md:   resource.NewMetadata(namespace, KubeletSpecType, id, resource.VersionUndefined),
		spec: &KubeletSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *KubeletSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *KubeletSpec) Spec() interface{} {
	return r.spec
}

func (r *KubeletSpec) String() string {
	return fmt.Sprintf("k8s.KubeletSpec(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *KubeletSpec) DeepCopy() resource.Resource {
	config := make(map[string]interface{}, len(r.spec.Config))

	for k, v := range r.spec.Config {
		config[k] = v
	}

	return &KubeletSpec{
		md: r.md,
		spec: &KubeletSpecSpec{
			Image:       r.spec.Image,
			Args:        append([]string(nil), r.spec.Args...),
			ExtraMounts: append([]specs.Mount(nil), r.spec.ExtraMounts...),
			Config:      config,
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *KubeletSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubeletSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

// TypedSpec returns .spec.
func (r *KubeletSpec) TypedSpec() *KubeletSpecSpec {
	return r.spec
}
