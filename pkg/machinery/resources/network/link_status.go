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

// LinkStatusType is type of LinkStatus resource.
const LinkStatusType = resource.Type("LinkStatuses.net.talos.dev")

// LinkStatus resource holds physical network link status.
type LinkStatus = typed.Resource[LinkStatusSpec, LinkStatusRD]

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
	BusPath          string                      `yaml:"busPath,omitempty"`
	PCIID            string                      `yaml:"pciID,omitempty"`
	Driver           string                      `yaml:"driver,omitempty"`
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

// Physical checks if the link is physical ethernet.
func (s LinkStatusSpec) Physical() bool {
	return s.Type == nethelpers.LinkEther && s.Kind == ""
}

// NewLinkStatus initializes a LinkStatus resource.
func NewLinkStatus(namespace resource.Namespace, id resource.ID) *LinkStatus {
	return typed.NewResource[LinkStatusSpec, LinkStatusRD](
		resource.NewMetadata(namespace, LinkStatusType, id, resource.VersionUndefined),
		LinkStatusSpec{},
	)
}

// LinkStatusRD provides auxiliary methods for LinkStatus.
type LinkStatusRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (LinkStatusRD) ResourceDefinition(resource.Metadata, LinkStatusSpec) meta.ResourceDefinitionSpec {
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
