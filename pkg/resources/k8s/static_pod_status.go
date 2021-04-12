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

// StaticPodStatusType is type of StaticPodStatus resource.
const StaticPodStatusType = resource.Type("StaticPodStatuses.kubernetes.talos.dev")

// StaticPodStatus resource holds definition of kubelet static pod.
type StaticPodStatus struct {
	md   resource.Metadata
	spec *staticPodStatusSpec
}

// staticPodStatusSpec describes kubelet static pod status.
type staticPodStatusSpec struct {
	*v1.PodStatus
}

func (spec *staticPodStatusSpec) MarshalYAML() (interface{}, error) {
	jsonSerialized, err := json.Marshal(spec.PodStatus)
	if err != nil {
		return nil, err
	}

	var obj interface{}

	err = json.Unmarshal(jsonSerialized, &obj)

	return obj, err
}

// NewStaticPodStatus initializes a StaticPodStatus resource.
func NewStaticPodStatus(namespace resource.Namespace, id resource.ID) *StaticPodStatus {
	r := &StaticPodStatus{
		md:   resource.NewMetadata(namespace, StaticPodStatusType, id, resource.VersionUndefined),
		spec: &staticPodStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *StaticPodStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *StaticPodStatus) Spec() interface{} {
	return r.spec
}

func (r *StaticPodStatus) String() string {
	return fmt.Sprintf("k8s.StaticPodStatus(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *StaticPodStatus) DeepCopy() resource.Resource {
	return &StaticPodStatus{
		md: r.md,
		spec: &staticPodStatusSpec{
			PodStatus: r.spec.PodStatus.DeepCopy(),
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *StaticPodStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StaticPodStatusType,
		Aliases:          []resource.Type{"podstatus"},
		DefaultNamespace: ControlPlaneNamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Ready",
				JSONPath: `{.conditions[?(@.type=="Ready")].status}`,
			},
		},
	}
}

// SetStatus sets pod status.
func (r *StaticPodStatus) SetStatus(status *v1.PodStatus) {
	r.spec.PodStatus = status
}
