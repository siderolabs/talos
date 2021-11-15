// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// HardwareAddrType is type of HardwareAddr resource.
const HardwareAddrType = resource.Type("HardwareAddresses.net.talos.dev")

// FirstHardwareAddr is a resource ID for the first NIC HW addr.
const FirstHardwareAddr = resource.ID("first")

// HardwareAddr resource describes hardware address of the physical links.
type HardwareAddr struct {
	md   resource.Metadata
	spec HardwareAddrSpec
}

// HardwareAddrSpec describes spec for the link.
type HardwareAddrSpec struct {
	// Name defines link name
	Name string `yaml:"name"`

	// Hardware address
	HardwareAddr nethelpers.HardwareAddr `yaml:"hardwareAddr"`
}

// NewHardwareAddr initializes a HardwareAddr resource.
func NewHardwareAddr(namespace resource.Namespace, id resource.ID) *HardwareAddr {
	r := &HardwareAddr{
		md:   resource.NewMetadata(namespace, HardwareAddrType, id, resource.VersionUndefined),
		spec: HardwareAddrSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *HardwareAddr) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *HardwareAddr) Spec() interface{} {
	return r.spec
}

func (r *HardwareAddr) String() string {
	return fmt.Sprintf("network.HardwareAddr(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *HardwareAddr) DeepCopy() resource.Resource {
	return &HardwareAddr{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *HardwareAddr) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             HardwareAddrType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *HardwareAddr) TypedSpec() *HardwareAddrSpec {
	return &r.spec
}
