// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package podman

import (
	"context"
	"fmt"
	"strconv"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v4/pkg/bindings/network"
	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/pkg/provision"
)

// createNetwork will take a network request and check if a network with the same name + cidr exists.
// If so, it simply returns without error and assumes we will re-use that network. Otherwise it will create a new one.
func (p *provisioner) createNetwork(ctx context.Context, req provision.NetworkRequest) error {
	existingNet, err := p.listNetworks(p.connection, req.Name)
	if err != nil {
		return err
	}

	// If named net already exists, see if we can reuse it
	if len(existingNet) > 0 {
		if existingNet[0].Subnets[0].Subnet.IPNet.String() != req.CIDRs[0].String() {
			return fmt.Errorf("existing network has differing cidr: %s vs %s", existingNet[0].Subnets[0].Subnet.IPNet.String(), req.CIDRs[0].String())
		}
		// CIDRs match, we'll reuse
		return nil
	}

	// Create new net
	ipnet, err := types.ParseCIDR(req.CIDRs[0].String())
	if err != nil {
		return err
	}

	options := types.Network{
		Name:        req.Name,
		Driver:      "bridge",
		Subnets:     []types.Subnet{{Subnet: ipnet}},
		DNSEnabled:  true,
		IPv6Enabled: false,
		Labels: map[string]string{
			"talos.owned":        "true",
			"talos.cluster.name": req.Name,
		},
		Options: map[string]string{
			"mtu": strconv.Itoa(req.MTU),
		},
	}

	_, err = network.Create(p.connection, &options)

	return err
}

func (p *provisioner) listNetworks(ctx context.Context, name string) ([]types.Network, error) {
	filters := map[string][]string{
		"label": {"talos.owned=true", "talos.cluster.name=" + name},
	}

	options := network.ListOptions{Filters: filters}

	return network.List(p.connection, &options)
}

func (p *provisioner) destroyNetwork(ctx context.Context, name string) error {
	networks, err := p.listNetworks(p.connection, name)
	if err != nil {
		return err
	}

	var result *multierror.Error

	for _, netw := range networks {
		if _, err := network.Remove(p.connection, netw.Name, &network.RemoveOptions{}); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}
