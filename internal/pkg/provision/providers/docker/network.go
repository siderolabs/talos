// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package docker

import (
	"context"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/pkg/provision"
)

func (p *provisioner) createNetwork(ctx context.Context, req provision.NetworkRequest) error {
	options := types.NetworkCreate{
		Labels: map[string]string{
			"talos.owned":        "true",
			"talos.cluster.name": req.Name,
		},
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{
				{
					Subnet: req.CIDR.String(),
				},
			},
		},
		Options: map[string]string{
			"com.docker.network.driver.mtu": strconv.Itoa(req.MTU),
		},
	}

	_, err := p.client.NetworkCreate(ctx, req.Name, options)

	return err
}

func (p *provisioner) listNetworks(ctx context.Context, name string) ([]types.NetworkResource, error) {
	filters := filters.NewArgs()
	filters.Add("label", "talos.owned=true")
	filters.Add("label", "talos.cluster.name="+name)

	options := types.NetworkListOptions{
		Filters: filters,
	}

	return p.client.NetworkList(ctx, options)
}

func (p *provisioner) destroyNetwork(ctx context.Context, name string) error {
	networks, err := p.listNetworks(ctx, name)
	if err != nil {
		return err
	}

	var result *multierror.Error

	for _, network := range networks {
		if err := p.client.NetworkRemove(ctx, network.ID); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}
