// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// StaticPodStatusType is type of StaticPodStatus resource.
const StaticPodStatusType = resource.Type("StaticPodStatuses.kubernetes.talos.dev")

// StaticPodStatus resource holds definition of kubelet static pod.
type StaticPodStatus = typed.Resource[StaticPodStatusSpec, StaticPodStatusExtension]

// StaticPodStatusSpec describes kubelet static pod status.
//
//gotagsrewrite:gen
type StaticPodStatusSpec struct {
	PodStatus map[string]any `protobuf:"1"`
}

// MarshalYAML implements yaml.Marshaler.
func (spec StaticPodStatusSpec) MarshalYAML() (any, error) {
	return spec.PodStatus, nil
}

// NewStaticPodStatus initializes a StaticPodStatus resource.
func NewStaticPodStatus(namespace resource.Namespace, id resource.ID) *StaticPodStatus {
	return typed.NewResource[StaticPodStatusSpec, StaticPodStatusExtension](
		resource.NewMetadata(namespace, StaticPodStatusType, id, resource.VersionUndefined),
		StaticPodStatusSpec{},
	)
}

// StaticPodStatusExtension provides auxiliary methods for StaticPodStatus.
type StaticPodStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (StaticPodStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
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
