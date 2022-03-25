// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
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
type KubeletLifecycle struct {
	md   resource.Metadata
	spec KubeletLifecycleSpec
}

// KubeletLifecycleSpec is empty.
type KubeletLifecycleSpec struct{}

// NewKubeletLifecycle initializes an empty KubeletLifecycle resource.
func NewKubeletLifecycle(namespace resource.Namespace, id resource.ID) *KubeletLifecycle {
	r := &KubeletLifecycle{
		md:   resource.NewMetadata(namespace, KubeletLifecycleType, id, resource.VersionUndefined),
		spec: KubeletLifecycleSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *KubeletLifecycle) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *KubeletLifecycle) Spec() interface{} {
	return r.spec
}

func (r *KubeletLifecycle) String() string {
	return fmt.Sprintf("k8s.KubeletLifecycle(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *KubeletLifecycle) DeepCopy() resource.Resource {
	return &KubeletLifecycle{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *KubeletLifecycle) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubeletLifecycleType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}
