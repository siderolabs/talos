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

// KubeletStatusType is type of KubeletStatus resource.
const KubeletStatusType = resource.Type("KubeletStatuses.kubernetes.talos.dev")

// KubeletStatus resource exposes the non-sensitive part of the kubelet runtime configuration.
type KubeletStatus = typed.Resource[KubeletStatusSpec, KubeletStatusExtension]

// KubeletStatusSpec describes the current kubelet state.
//
//gotagsrewrite:gen
type KubeletStatusSpec struct {
	// Image is the kubelet image reference.
	Image string `yaml:"image" protobuf:"1"`
}

// NewKubeletStatus initializes an empty KubeletStatus resource.
func NewKubeletStatus(namespace resource.Namespace, id resource.ID) *KubeletStatus {
	return typed.NewResource[KubeletStatusSpec, KubeletStatusExtension](
		resource.NewMetadata(namespace, KubeletStatusType, id, resource.VersionUndefined),
		KubeletStatusSpec{},
	)
}

// KubeletStatusExtension provides auxiliary methods for KubeletStatus.
type KubeletStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubeletStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubeletStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Image",
				JSONPath: "{.image}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KubeletStatusSpec](KubeletStatusType, &KubeletStatus{})
	if err != nil {
		panic(err)
	}
}
