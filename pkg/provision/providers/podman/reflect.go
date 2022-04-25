// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package podman

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/containers/podman/v4/pkg/bindings/containers"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/provision"
)

func (p *provisioner) Reflect(ctx context.Context, clusterName, stateDirectory string) (provision.Cluster, error) {
	res := &result{
		clusterInfo: provision.ClusterInfo{
			ClusterName: clusterName,
		},
	}

	// find network assuming network name == cluster name
	networks, err := p.listNetworks(p.connection, clusterName)
	if err != nil {
		return nil, err
	}

	if len(networks) > 0 {
		network := networks[0]

		var cidr *net.IPNet
		_, cidr, err = net.ParseCIDR(network.Subnets[0].Subnet.String())

		if err != nil {
			return nil, err
		}

		res.clusterInfo.Network.Name = network.Name
		res.clusterInfo.Network.CIDRs = []net.IPNet{*cidr}
		res.clusterInfo.Network.GatewayAddrs = []net.IP{network.Subnets[0].Gateway}

		mtuStr := network.Options["mtu"]
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

		info, err := containers.Inspect(p.connection, node.Names[0], &containers.InspectOptions{})
		if err != nil {
			return nil, err
		}

		res.clusterInfo.Nodes = append(res.clusterInfo.Nodes,
			provision.NodeInfo{
				ID:   node.ID,
				Name: strings.TrimLeft(node.Names[0], "/"),
				Type: t,

				IPs: []net.IP{net.ParseIP(info.NetworkSettings.Networks[res.clusterInfo.Network.Name].IPAddress)},
			})
	}

	return res, nil
}
