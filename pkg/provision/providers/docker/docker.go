// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package docker implements Provisioner via docker.
package docker

import (
	"context"
	"runtime"

	"github.com/docker/docker/client"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/provision"
)

type provisioner struct {
	client *client.Client
}

// NewProvisioner initializes docker provisioner.
func NewProvisioner(ctx context.Context) (provision.Provisioner, error) {
	p := &provisioner{}

	var err error

	p.client, err = client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return p, nil
}

// Close and release resources.
func (p *provisioner) Close() error {
	if p.client != nil {
		return p.client.Close()
	}

	return nil
}

// GenOptions provides a list of additional config generate options.
func (p *provisioner) GenOptions(networkReq provision.NetworkRequest) []generate.GenOption {
	ret := []generate.GenOption{
		generate.WithPersist(false),
	}

	networkConfig := &v1alpha1.NetworkConfig{
		NetworkInterfaces: []*v1alpha1.Device{
			{
				DeviceInterface: "eth0",
				DeviceIgnore:    true,
			},
		},
	}

	if len(networkReq.Nameservers) > 0 {
		nameservers := make([]string, len(networkReq.Nameservers))
		for i := range nameservers {
			nameservers[i] = networkReq.Nameservers[i].String()
		}

		networkConfig.NameServers = nameservers
	}

	ret = append(ret, generate.WithNetworkConfig(networkConfig))

	return ret
}

// GetLoadBalancers returns internal/external loadbalancer endpoints.
func (p *provisioner) GetLoadBalancers(networkReq provision.NetworkRequest) (internalEndpoint, externalEndpoint string) {
	// docker doesn't provide internal LB, so return empty string
	// external LB is always localhost for OS X where docker exposes ports
	switch runtime.GOOS {
	case "darwin":
		return "", "127.0.0.1"
	default:
		return "", ""
	}
}
