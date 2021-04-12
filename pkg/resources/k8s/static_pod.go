// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"encoding/json"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	v1 "k8s.io/api/core/v1"
)

// StaticPodType is type of StaticPod resource.
const StaticPodType = resource.Type("StaticPods.kubernetes.talos.dev")

// StaticPod resource holds definition of kubelet static pod.
type StaticPod struct {
	md   resource.Metadata
	spec *staticPodSpec
}

type staticPodSpec struct {
	*v1.Pod
}

func (spec *staticPodSpec) MarshalYAML() (interface{}, error) {
	jsonSerialized, err := json.Marshal(spec.Pod)
	if err != nil {
		return nil, err
	}

	var obj interface{}

	err = json.Unmarshal(jsonSerialized, &obj)

	return obj, err
}

// NewStaticPod initializes a StaticPod resource.
func NewStaticPod(namespace resource.Namespace, id resource.ID, spec *v1.Pod) *StaticPod {
	r := &StaticPod{
		md: resource.NewMetadata(namespace, StaticPodType, id, resource.VersionUndefined),
		spec: &staticPodSpec{
			Pod: spec,
		},
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
		spec: &staticPodSpec{
			Pod: r.spec.Pod.DeepCopy(),
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *StaticPod) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StaticPodType,
		Aliases:          []resource.Type{},
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}

// Pod returns pod definition.
func (r *StaticPod) Pod() *v1.Pod {
	return r.spec.Pod
}

// SetPod sets pod definition.
func (r *StaticPod) SetPod(podSpec *v1.Pod) {
	r.spec.Pod = podSpec
}
