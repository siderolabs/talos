// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// StaticPodType is type of StaticPod resource.
const StaticPodType = resource.Type("StaticPods.kubernetes.talos.dev")

// StaticPod resource holds definition of kubelet static pod.
type StaticPod struct {
	md   resource.Metadata
	spec *StaticPodSpec
}

// StaticPodSpec describes static pod spec, it contains marshaled *v1.Pod spec.
type StaticPodSpec struct {
	Pod map[string]interface{}
}

// MarshalYAML implements yaml.Marshaler.
func (spec *StaticPodSpec) MarshalYAML() (interface{}, error) {
	return spec.Pod, nil
}

// NewStaticPod initializes a StaticPod resource.
func NewStaticPod(namespace resource.Namespace, id resource.ID) *StaticPod {
	r := &StaticPod{
		md:   resource.NewMetadata(namespace, StaticPodType, id, resource.VersionUndefined),
		spec: &StaticPodSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *StaticPod) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *StaticPod) Spec() interface{} {
	return r.spec
}

func (r *StaticPod) String() string {
	return fmt.Sprintf("k8s.StaticPod(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *StaticPod) DeepCopy() resource.Resource {
	return &StaticPod{
		md: r.md,
		spec: &StaticPodSpec{
			Pod: r.spec.Pod,
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *StaticPod) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StaticPodType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

// TypedSpec returns .spec.
func (r *StaticPod) TypedSpec() *StaticPodSpec {
	return r.spec
}
