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

// KubeletCPUReservationType is type of KubeletCPUReservation resource.
const KubeletCPUReservationType = resource.Type("KubeletCPUReservations.kubernetes.talos.dev")

// KubeletCPUReservation is the CPU partition coordinator's staged command for the kubelet's
// reservedSystemCPUs.
//
// It is always published once the CPU partition policy is projected, so the kubelet renderer
// can tell "coordinator not running yet" (resource missing) from "kubelet not managed"
// (Managed false). It changes only when the staged value changes.
type KubeletCPUReservation = typed.Resource[KubeletCPUReservationSpec, KubeletCPUReservationExtension]

// KubeletCPUReservationSpec describes the staged reservation.
//
//gotagsrewrite:gen
type KubeletCPUReservationSpec struct {
	// Managed is true while the CPU partition bounds kubepods; ReservedCPUs is then authoritative.
	Managed bool `yaml:"managed" protobuf:"1"`
	// ReservedCPUs is the reservedSystemCPUs list to render, empty when unmanaged.
	ReservedCPUs string `yaml:"reservedCPUs,omitempty" protobuf:"2"`
}

// NewKubeletCPUReservation initializes a KubeletCPUReservation resource.
func NewKubeletCPUReservation() *KubeletCPUReservation {
	return typed.NewResource[KubeletCPUReservationSpec, KubeletCPUReservationExtension](
		resource.NewMetadata(NamespaceName, KubeletCPUReservationType, KubeletID, resource.VersionUndefined),
		KubeletCPUReservationSpec{},
	)
}

// KubeletCPUReservationExtension is auxiliary resource data for KubeletCPUReservation.
type KubeletCPUReservationExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (KubeletCPUReservationExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubeletCPUReservationType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Managed",
				JSONPath: `{.managed}`,
			},
			{
				Name:     "Reserved",
				JSONPath: `{.reservedCPUs}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KubeletCPUReservationSpec](KubeletCPUReservationType, &KubeletCPUReservation{})
	if err != nil {
		panic(err)
	}
}
