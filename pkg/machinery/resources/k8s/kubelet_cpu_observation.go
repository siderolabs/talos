// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// KubeletCPUObservationType is the type of KubeletCPUObservation.
const KubeletCPUObservationType = resource.Type("KubeletCPUObservations.kubernetes.talos.dev")

// KubeletSpecTokenAnnotation binds an OCI argument snapshot to its source resource.
const KubeletSpecTokenAnnotation = "kubernetes.talos.dev/kubelet-spec-token"

// KubeletCPUManagedAnnotation records CPU ownership in the rendered and OCI snapshots.
const KubeletCPUManagedAnnotation = "kubernetes.talos.dev/kubelet-cpu-managed"

// KubeletCPUManaged reports ownership, independent of legacy CPU argument values.
func KubeletCPUManaged(spec *KubeletSpec) bool {
	value, _ := spec.Metadata().Annotations().Get(KubeletCPUManagedAnnotation)

	return value == "true"
}

// KubeletSpecToken identifies a rendered spec, including singleton recreation.
func KubeletSpecToken(spec *KubeletSpec) string {
	return spec.Metadata().Created().UTC().Format(time.RFC3339Nano) + "/" + spec.Metadata().Version().String()
}

// KubeletCPUObservation records a task's immutable argument binding, not current health.
type KubeletCPUObservation = typed.Resource[KubeletCPUObservationSpec, KubeletCPUObservationExtension]

// KubeletCPUObservationSpec contains only the observed CPU binding.
//
//gotagsrewrite:gen
type KubeletCPUObservationSpec struct {
	SpecToken            string `yaml:"specToken" protobuf:"1"`
	Managed              bool   `yaml:"managed" protobuf:"2"`
	ReservedCPUs         string `yaml:"reservedCPUs" protobuf:"3"`
	CPUManagerPolicy     string `yaml:"cpuManagerPolicy" protobuf:"4"`
	StrictCPUReservation bool   `yaml:"strictCPUReservation" protobuf:"5"`
	ContainerCreated     string `yaml:"containerCreated" protobuf:"6"`
	TaskPID              uint32 `yaml:"taskPID" protobuf:"7"`
}

// NewKubeletCPUObservation initializes an observation.
func NewKubeletCPUObservation() *KubeletCPUObservation {
	return typed.NewResource[KubeletCPUObservationSpec, KubeletCPUObservationExtension](
		resource.NewMetadata(NamespaceName, KubeletCPUObservationType, KubeletID, resource.VersionUndefined),
		KubeletCPUObservationSpec{},
	)
}

// KubeletCPUObservationExtension provides the resource definition.
type KubeletCPUObservationExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider.
func (KubeletCPUObservationExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{Type: KubeletCPUObservationType, DefaultNamespace: NamespaceName}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic[KubeletCPUObservationSpec](KubeletCPUObservationType, &KubeletCPUObservation{}); err != nil {
		panic(err)
	}
}
