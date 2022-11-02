// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package docker

import (
	"context"
	"net/netip"
	"strconv"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/siderolabs/talos/pkg/provision"
)

func (p *provisioner) Reflect(ctx context.Context, clusterName, stateDirectory string) (provision.Cluster, error) {
	res := &result{
		clusterInfo: provision.ClusterInfo{
			ClusterName: clusterName,
		},
	}

	// find network assuming network name == cluster name
	networks, err := p.listNetworks(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	if len(networks) > 0 {
		network := networks[0]

		var cidr netip.Prefix

		cidr, err = netip.ParsePrefix(network.IPAM.Config[0].Subnet)
		if err != nil {
			return nil, err
		}

		res.clusterInfo.Network.Name = network.Name
		res.clusterInfo.Network.CIDRs = []netip.Prefix{cidr}
		res.clusterInfo.Network.GatewayAddrs = []netip.Addr{}

		var addr netip.Addr

		if addr, err = netip.ParseAddr(network.IPAM.Config[0].Gateway); err == nil {
			res.clusterInfo.Network.GatewayAddrs = append(res.clusterInfo.Network.GatewayAddrs, addr)
		}

		mtuStr := network.Options["com.docker.network.driver.mtu"]
		res.clusterInfo.Network.MTU, err = strconv.Atoi(mtuStr)

		if err != nil {
			return nil, err
		}
	}

	// find nodes (containers)
	nodes, err := p.listNodes(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		t, err := machine.ParseType(node.Labels["talos.type"])
		if err != nil {
			return nil, err
		}

		addr, err := netip.ParseAddr(node.NetworkSettings.Networks[res.clusterInfo.Network.Name].IPAddress)
		if err != nil {
			return nil, err
		}

		res.clusterInfo.Nodes = append(res.clusterInfo.Nodes,
			provision.NodeInfo{
				ID:   node.ID,
				Name: strings.TrimLeft(node.Names[0], "/"),
				Type: t,

				IPs: []netip.Addr{addr},
			})
	}

	return res, nil
}
