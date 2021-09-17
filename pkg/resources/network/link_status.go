// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package network

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// LinkStatus resource holds physical network link status.
type LinkStatus struct {
	md   resource.Metadata
	spec LinkStatusSpec
}

// LinkStatusSpec describes status of rendered secrets.
type LinkStatusSpec struct {
	// Fields coming from rtnetlink API.
	Index            uint32                      `yaml:"index"`
	Type             nethelpers.LinkType         `yaml:"type"`
	LinkIndex        uint32                      `yaml:"linkIndex"`
	Flags            nethelpers.LinkFlags        `yaml:"flags"`
	HardwareAddr     nethelpers.HardwareAddr     `yaml:"hardwareAddr"`
	BroadcastAddr    nethelpers.HardwareAddr     `yaml:"broadcastAddr"`
	MTU              uint32                      `yaml:"mtu"`
	QueueDisc        string                      `yaml:"queueDisc"`
	MasterIndex      uint32                      `yaml:"masterIndex,omitempty"`
	OperationalState nethelpers.OperationalState `yaml:"operationalState"`
	Kind             string                      `yaml:"kind"`
	SlaveKind        string                      `yaml:"slaveKind"`
	// Fields coming from ethtool API.
	LinkState     bool              `yaml:"linkState"`
	SpeedMegabits int               `yaml:"speedMbit,omitempty"`
	Port          nethelpers.Port   `yaml:"port"`
	Duplex        nethelpers.Duplex `yaml:"duplex"`
	// Following fields are only populated with respective Kind.
	VLAN       VLANSpec       `yaml:"vlan,omitempty"`
	BondMaster BondMasterSpec `yaml:"bondMaster,omitempty"`
	Wireguard  WireguardSpec  `yaml:"wireguard,omitempty"`
}

// NewLinkStatus initializes a LinkStatus resource.
func NewLinkStatus(namespace resource.Namespace, id resource.ID) *LinkStatus {
	r := &LinkStatus{
		md:   resource.NewMetadata(namespace, LinkStatusType, id, resource.VersionUndefined),
		spec: LinkStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *LinkStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *LinkStatus) Spec() interface{} {
	return r.spec
}

func (r *LinkStatus) String() string {
	return fmt.Sprintf("network.LinkStatus(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *LinkStatus) DeepCopy() resource.Resource {
	return &LinkStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *LinkStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LinkStatusType,
		Aliases:          []resource.Type{"link", "links"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Type",
				JSONPath: `{.type}`,
			},
			{
				Name:     "Kind",
				JSONPath: `{.kind}`,
			},
			{
				Name:     "Hw Addr",
				JSONPath: `{.hardwareAddr}`,
			},
			{
				Name:     "Oper State",
				JSONPath: `{.operationalState}`,
			},
			{
				Name:     "Link State",
				JSONPath: `{.linkState}`,
			},
		},
		Sensitivity: meta.NonSensitive,
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *LinkStatus) TypedSpec() *LinkStatusSpec {
	return &r.spec
}

// Physical checks if the link is physical ethernet.
func (r *LinkStatus) Physical() bool {
	return r.TypedSpec().Type == nethelpers.LinkEther && r.TypedSpec().Kind == ""
}
