// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package docker

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/siderolabs/talos/pkg/provision"
)

// createNetwork will take a network request and check if a network with the same name + cidr exists.
// If so, it simply returns without error and assumes we will re-use that network. Otherwise it will create a new one.
func (p *provisioner) createNetwork(ctx context.Context, req provision.NetworkRequest) error {
	existingNet, err := p.listNetworks(ctx, req.Name)
	if err != nil {
		return err
	}

	// If named net already exists, see if we can reuse it
	if len(existingNet) > 0 {
		if existingNet[0].IPAM.Config[0].Subnet != req.CIDRs[0] {
			return fmt.Errorf("existing network has differing cidr: %s vs %s", existingNet[0].IPAM.Config[0].Subnet, req.CIDRs[0].String())
		}
		// CIDRs match, we'll reuse
		return nil
	}

	// Create new net
	options := client.NetworkCreateOptions{
		Labels: map[string]string{
			"talos.owned":        "true",
			"talos.cluster.name": req.Name,
		},
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{
				{
					Subnet: req.CIDRs[0],
				},
			},
		},
		Options: map[string]string{
			"com.docker.network.driver.mtu": strconv.Itoa(req.MTU),
		},
	}

	_, err = p.client.NetworkCreate(ctx, req.Name, options)

	return err
}

func (p *provisioner) listNetworks(ctx context.Context, name string) ([]network.Summary, error) {
	filters := client.Filters{}
	filters.Add("label", "talos.owned=true")
	filters.Add("label", "talos.cluster.name="+name)

	options := client.NetworkListOptions{
		Filters: filters,
	}

	result, err := p.client.NetworkList(ctx, options)
	if err != nil {
		return nil, err
	}

	return result.Items, nil
}

func (p *provisioner) destroyNetwork(ctx context.Context, name string) error {
	networks, err := p.listNetworks(ctx, name)
	if err != nil {
		return err
	}

	var result *multierror.Error

	for _, network := range networks {
		if _, err := p.client.NetworkRemove(ctx, network.ID, client.NetworkRemoveOptions{}); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}
