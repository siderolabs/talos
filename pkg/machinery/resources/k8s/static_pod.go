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

// StaticPodType is type of StaticPod resource.
const StaticPodType = resource.Type("StaticPods.kubernetes.talos.dev")

// StaticPod resource holds definition of kubelet static pod.
type StaticPod = typed.Resource[StaticPodSpec, StaticPodExtension]

// StaticPodSpec describes static pod spec, it contains marshaled *v1.Pod spec.
//
//gotagsrewrite:gen
type StaticPodSpec struct {
	Pod map[string]any `protobuf:"1"`
}

// MarshalYAML implements yaml.Marshaler.
func (spec StaticPodSpec) MarshalYAML() (any, error) {
	return spec.Pod, nil
}

// NewStaticPod initializes a StaticPod resource.
func NewStaticPod(namespace resource.Namespace, id resource.ID) *StaticPod {
	return typed.NewResource[StaticPodSpec, StaticPodExtension](
		resource.NewMetadata(namespace, StaticPodType, id, resource.VersionUndefined),
		StaticPodSpec{},
	)
}

// StaticPodExtension provides auxiliary methods for StaticPod.
type StaticPodExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (StaticPodExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StaticPodType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[StaticPodSpec](StaticPodType, &StaticPod{})
	if err != nil {
		panic(err)
	}
}
