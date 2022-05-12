// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// HardwareAddrType is type of HardwareAddr resource.
const HardwareAddrType = resource.Type("HardwareAddresses.net.talos.dev")

// FirstHardwareAddr is a resource ID for the first NIC HW addr.
const FirstHardwareAddr = resource.ID("first")

// HardwareAddr resource describes hardware address of the physical links.
type HardwareAddr = typed.Resource[HardwareAddrSpec, HardwareAddrRD]

// HardwareAddrSpec describes spec for the link.
type HardwareAddrSpec struct {
	// Name defines link name
	Name string `yaml:"name"`

	// Hardware address
	HardwareAddr nethelpers.HardwareAddr `yaml:"hardwareAddr"`
}

// NewHardwareAddr initializes a HardwareAddr resource.
func NewHardwareAddr(namespace resource.Namespace, id resource.ID) *HardwareAddr {
	return typed.NewResource[HardwareAddrSpec, HardwareAddrRD](
		resource.NewMetadata(namespace, HardwareAddrType, id, resource.VersionUndefined),
		HardwareAddrSpec{},
	)
}

// HardwareAddrRD provides auxiliary methods for HardwareAddr.
type HardwareAddrRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (HardwareAddrRD) ResourceDefinition(resource.Metadata, HardwareAddrSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             HardwareAddrType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}
