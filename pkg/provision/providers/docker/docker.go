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

	p.client, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
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
	nameservers := make([]string, 0, len(networkReq.Nameservers))

	hasV4 := false
	hasV6 := false

	for _, subnet := range networkReq.CIDRs {
		if subnet.IP.To4() == nil {
			hasV6 = true
		} else {
			hasV4 = true
		}
	}

	// filter nameservers by IPv4/IPv6
	for i := range networkReq.Nameservers {
		if networkReq.Nameservers[i].To4() == nil && hasV6 {
			nameservers = append(nameservers, networkReq.Nameservers[i].String())
		} else if networkReq.Nameservers[i].To4() != nil && hasV4 {
			nameservers = append(nameservers, networkReq.Nameservers[i].String())
		}
	}

	return []generate.GenOption{
		generate.WithPersist(false),
		generate.WithNetworkOptions(
			v1alpha1.WithNetworkInterfaceIgnore("eth0"),
			v1alpha1.WithNetworkNameservers(nameservers...),
		),
	}
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

// UserDiskName not implemented for docker.
func (p *provisioner) UserDiskName(index int) string {
	return ""
}

// GetFirstInterface returns first network interface name.
func (p *provisioner) GetFirstInterface() string {
	return "eth0"
}
