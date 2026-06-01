// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"net/netip"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/provision"
)

// MapProvisionNodeInfosToClusterNodeInfos maps provision.NodeInfos to cluster.NodeInfos.
func MapProvisionNodeInfosToClusterNodeInfos(nodes []provision.NodeInfo) ([]NodeInfo, error) {
	result := make([]NodeInfo, len(nodes))

	for i, info := range nodes {
		clusterNodeInfo, err := toClusterNodeInfo(info)
		if err != nil {
			return nil, err
		}

		result[i] = *clusterNodeInfo
	}

	return result, nil
}

// MapProvisionNodeInfosToNodeInfosByType maps provision.NodeInfos
// to cluster.NodeInfos, grouping them by machine type.
func MapProvisionNodeInfosToNodeInfosByType(nodes []provision.NodeInfo) (map[machine.Type][]NodeInfo, error) {
	result := make(map[machine.Type][]NodeInfo)

	for _, info := range nodes {
		clusterNodeInfo, err := toClusterNodeInfo(info)
		if err != nil {
			return nil, err
		}

		result[info.Type] = append(result[info.Type], *clusterNodeInfo)
	}

	return result, nil
}

func toClusterNodeInfo(info provision.NodeInfo) (*NodeInfo, error) {
	ips := make([]netip.Addr, len(info.IPs))

	for i, ip := range info.IPs {
		parsed, err := netip.ParseAddr(ip.String())
		if err != nil {
			return nil, err
		}

		ips[i] = parsed
	}

	return &NodeInfo{
		InternalIP: ips[0],
		IPs:        ips,
	}, nil
}
