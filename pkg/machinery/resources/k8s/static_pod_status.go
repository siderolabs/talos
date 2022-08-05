// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/talos-systems/talos/pkg/machinery/proto"
)

// StaticPodStatusType is type of StaticPodStatus resource.
const StaticPodStatusType = resource.Type("StaticPodStatuses.kubernetes.talos.dev")

// StaticPodStatus resource holds definition of kubelet static pod.
type StaticPodStatus = typed.Resource[StaticPodStatusSpec, StaticPodStatusRD]

// StaticPodStatusSpec describes kubelet static pod status.
//
//gotagsrewrite:gen
type StaticPodStatusSpec struct {
	PodStatus map[string]interface{} `protobuf:"1"`
}

// MarshalYAML implements yaml.Marshaler.
func (spec StaticPodStatusSpec) MarshalYAML() (interface{}, error) {
	return spec.PodStatus, nil
}

// NewStaticPodStatus initializes a StaticPodStatus resource.
func NewStaticPodStatus(namespace resource.Namespace, id resource.ID) *StaticPodStatus {
	return typed.NewResource[StaticPodStatusSpec, StaticPodStatusRD](
		resource.NewMetadata(namespace, StaticPodStatusType, id, resource.VersionUndefined),
		StaticPodStatusSpec{},
	)
}

// StaticPodStatusRD provides auxiliary methods for StaticPodStatus.
type StaticPodStatusRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (StaticPodStatusRD) ResourceDefinition(resource.Metadata, StaticPodStatusSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StaticPodStatusType,
		Aliases:          []resource.Type{"podstatus"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Ready",
				JSONPath: `{.conditions[?(@.type=="Ready")].status}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[StaticPodStatusSpec](StaticPodStatusType, &StaticPodStatus{})
	if err != nil {
		panic(err)
	}
}
