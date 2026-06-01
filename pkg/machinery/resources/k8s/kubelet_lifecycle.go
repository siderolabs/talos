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

// KubeletLifecycleType is type of KubeletLifecycle resource.
const KubeletLifecycleType = resource.Type("KubeletLifecycles.kubernetes.talos.dev")

// KubeletLifecycleID is the singleton ID of the resource.
const KubeletLifecycleID = resource.ID("kubelet")

// KubeletLifecycle resource exists to signal that the kubelet pods are running.
//
// Components might put finalizers on the KubeletLifecycle resource to signal that additional
// actions should be taken before the kubelet is about to be shut down.
//
// KubeletLifecycle is mostly about status of the workloads kubelet is running vs.
// the actual status of the kubelet service itself.
type KubeletLifecycle = typed.Resource[KubeletLifecycleSpec, KubeletLifecycleExtension]

// KubeletLifecycleSpec is empty.
type KubeletLifecycleSpec struct{}

// NewKubeletLifecycle initializes an empty KubeletLifecycle resource.
func NewKubeletLifecycle(namespace resource.Namespace, id resource.ID) *KubeletLifecycle {
	return typed.NewResource[KubeletLifecycleSpec, KubeletLifecycleExtension](
		resource.NewMetadata(namespace, KubeletLifecycleType, id, resource.VersionUndefined),
		KubeletLifecycleSpec{},
	)
}

// KubeletLifecycleExtension provides auxiliary methods for KubeletLifecycle.
type KubeletLifecycleExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubeletLifecycleExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubeletLifecycleType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KubeletLifecycleSpec](KubeletLifecycleType, &KubeletLifecycle{})
	if err != nil {
		panic(err)
	}
}
