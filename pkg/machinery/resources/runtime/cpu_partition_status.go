// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// CPUPartitionStatusType is type of CPUPartitionStatus resource.
const CPUPartitionStatusType = resource.Type("CPUPartitionStatuses.runtime.talos.dev")

// CPUPartitionStatus is the outcome of applying the CPU partition policy of the boot.
//
// It is published once the policy is applied or has failed, or right away when there is no policy.
// The kubelet, Talos containers and virtual machines start once it is ready; system services start
// before it and are never held.
type CPUPartitionStatus = typed.Resource[CPUPartitionStatusSpec, CPUPartitionStatusExtension]

// CPUPartitionStatusID is the ID of the singleton CPUPartitionStatus.
const CPUPartitionStatusID = CPUPartitionSpecID

// CPUPartitionStatusSpec describes the applied CPU partition policy.
//
//gotagsrewrite:gen
type CPUPartitionStatusSpec struct {
	Ready bool `yaml:"ready" protobuf:"1"`
	// KubeletReservedCPUs is set when kubepods is bounded: the online CPUs outside kubepods.
	KubeletReservedCPUs string `yaml:"kubeletReservedCPUs,omitempty" protobuf:"2"`
	// VirtualMachinePools is set when the virtual machine root is bounded.
	VirtualMachinePools []CPUPartitionPoolSpec `yaml:"virtualMachinePools,omitempty" protobuf:"3"`
	// Error is why the policy failed to apply.
	Error string `yaml:"error,omitempty" protobuf:"4"`
}

// CPUPartitionPoolSpec is one pool of the virtual machine root.
//
//gotagsrewrite:gen
type CPUPartitionPoolSpec struct {
	// Name is the slice name, or "shared" for the CPUs outside every named slice.
	Name string `yaml:"name" protobuf:"1"`
	CPUs string `yaml:"cpus" protobuf:"2"`
	// Partition is the libvirt resource partition of the pool.
	Partition string `yaml:"partition" protobuf:"3"`
}

// NewCPUPartitionStatus initializes a CPUPartitionStatus resource.
func NewCPUPartitionStatus() *CPUPartitionStatus {
	return typed.NewResource[CPUPartitionStatusSpec, CPUPartitionStatusExtension](
		resource.NewMetadata(NamespaceName, CPUPartitionStatusType, CPUPartitionStatusID, resource.VersionUndefined),
		CPUPartitionStatusSpec{},
	)
}

// ReadyCPUPartitionStatus returns the CPU partition status of the boot once it is ready, nil before.
//
// The status only ever moves from missing or failed to ready, once per boot.
func ReadyCPUPartitionStatus(ctx context.Context, r controller.Reader) (*CPUPartitionStatusSpec, error) {
	status, err := safe.ReaderGetByID[*CPUPartitionStatus](ctx, r, CPUPartitionStatusID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error getting CPU partition status: %w", err)
	}

	if !status.TypedSpec().Ready {
		return nil, nil
	}

	return status.TypedSpec(), nil
}

// CPUPartitionStatusExtension is auxiliary resource data for CPUPartitionStatus.
type CPUPartitionStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (CPUPartitionStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             CPUPartitionStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Ready",
				JSONPath: `{.ready}`,
			},
			{
				Name:     "Error",
				JSONPath: `{.error}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[CPUPartitionStatusSpec](CPUPartitionStatusType, &CPUPartitionStatus{})
	if err != nil {
		panic(err)
	}
}
