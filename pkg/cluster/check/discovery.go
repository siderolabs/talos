// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"net/netip"

	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/generic/maps"
	"github.com/talos-systems/talos/pkg/machinery/generic/slices"
	clussterres "github.com/talos-systems/talos/pkg/machinery/resources/cluster"
)

// DiscoveredClusterInfo represents a cluster.Info populated using the discovery service.
type DiscoveredClusterInfo struct {
	nodes       []cluster.NodeInfo
	nodesByType map[machine.Type][]cluster.NodeInfo
}

// Nodes returns list of all node infos.
func (d *DiscoveredClusterInfo) Nodes() []cluster.NodeInfo {
	return d.nodes
}

// NodesByType return list of node endpoints by type.
func (d *DiscoveredClusterInfo) NodesByType(m machine.Type) []cluster.NodeInfo {
	return d.nodesByType[m]
}

// NewDiscoveredClusterInfo returns a new cluster.Info populated from the discovery service.
func NewDiscoveredClusterInfo(members []clussterres.Member) (cluster.Info, error) {
	m, err := membersToNodeInfoMap(members)
	if err != nil {
		return nil, err
	}

	nodes := slices.FlatMap(maps.Values(m), func(t []cluster.NodeInfo) []cluster.NodeInfo { return t })

	return &DiscoveredClusterInfo{
		nodes:       nodes,
		nodesByType: m,
	}, nil
}

func membersToNodeInfoMap(members []clussterres.Member) (map[machine.Type][]cluster.NodeInfo, error) {
	result := make(map[machine.Type][]cluster.NodeInfo)

	for _, member := range members {
		spec := member.TypedSpec()

		machineType := spec.MachineType

		nodeInfo, err := memberToNodeInfo(&member)
		if err != nil {
			return nil, err
		}

		result[machineType] = append(result[machineType], *nodeInfo)
	}

	return result, nil
}

func memberToNodeInfo(member *clussterres.Member) (*cluster.NodeInfo, error) {
	ips, err := stringsToNetipAddrs(slices.Map(member.TypedSpec().Addresses, netaddrIPToString))
	if err != nil {
		return nil, err
	}

	return &cluster.NodeInfo{
		InternalIP: ips[0],
		IPs:        ips,
	}, nil
}

func netaddrIPToString(ip netaddr.IP) string {
	return ip.String()
}

func stringsToNetipAddrs(ips []string) ([]netip.Addr, error) {
	result := make([]netip.Addr, len(ips))

	for i, ip := range ips {
		parsed, err := netip.ParseAddr(ip)
		if err != nil {
			return nil, err
		}

		result[i] = parsed
	}

	return result, nil
}
