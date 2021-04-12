// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// MachineTypeType is type of MachineType resource.
const MachineTypeType = resource.Type("MachineTypes.config.talos.dev")

// MachineTypeID is singleton resource ID.
const MachineTypeID = resource.ID("machine-type")

// MachineType describes machine type.
type MachineType struct {
	md   resource.Metadata
	spec machineTypeSpec
}

type machineTypeSpec struct {
	machine.Type
}

func (spec machineTypeSpec) MarshalYAML() (interface{}, error) {
	return spec.Type.String(), nil
}

// NewMachineType initializes a MachineType resource.
func NewMachineType() *MachineType {
	r := &MachineType{
		md:   resource.NewMetadata(NamespaceName, MachineTypeType, MachineTypeID, resource.VersionUndefined),
		spec: machineTypeSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *MachineType) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *MachineType) Spec() interface{} {
	return r.spec
}

func (r *MachineType) String() string {
	return fmt.Sprintf("config.MachineType(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *MachineType) DeepCopy() resource.Resource {
	return &MachineType{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *MachineType) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MachineTypeType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Type",
				JSONPath: "{@}",
			},
		},
	}
}

// MachineType returns machine.Type.
func (r *MachineType) MachineType() machine.Type {
	return r.spec.Type
}

// SetMachineType sets machine.Type.
func (r *MachineType) SetMachineType(typ machine.Type) {
	r.spec.Type = typ
}
