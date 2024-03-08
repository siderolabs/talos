// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package docker implements Provisioner via docker.
package docker

import (
	"bytes"
	"context"
	"os"
	"runtime"

	"github.com/docker/docker/client"

	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/provision"
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
func (p *provisioner) GenOptions(networkReq provision.NetworkRequest) []generate.Option {
	nameservers := make([]string, 0, len(networkReq.Nameservers))

	hasV4 := false
	hasV6 := false

	for _, subnet := range networkReq.CIDRs {
		if subnet.Addr().Is6() {
			hasV6 = true
		} else {
			hasV4 = true
		}
	}

	// filter nameservers by IPv4/IPv6
	for i := range networkReq.Nameservers {
		if networkReq.Nameservers[i].Is6() && hasV6 {
			nameservers = append(nameservers, networkReq.Nameservers[i].String())
		} else if networkReq.Nameservers[i].Is4() && hasV4 {
			nameservers = append(nameservers, networkReq.Nameservers[i].String())
		}
	}

	return []generate.Option{
		generate.WithNetworkOptions(
			v1alpha1.WithNetworkInterfaceIgnore(v1alpha1.IfaceByName("eth0")),
			v1alpha1.WithNetworkNameservers(nameservers...),
		),
	}
}

// GetLoadBalancers returns internal/external loadbalancer endpoints.
func (p *provisioner) GetLoadBalancers(networkReq provision.NetworkRequest) (internalEndpoint, externalEndpoint string) {
	// docker doesn't provide internal LB, so return empty string
	// external LB is always localhost for OS X where docker exposes ports
	switch runtime.GOOS {
	case "darwin", "windows":
		return "", "127.0.0.1"
	case "linux":
		// if detectWSL() {
		return "", "127.0.0.1"
		// }

		// fallthrough
	default:
		return "", ""
	}
}

// UserDiskName not implemented for docker.
func (p *provisioner) UserDiskName(index int) string {
	return ""
}

// GetFirstInterface returns first network interface name.
func (p *provisioner) GetFirstInterface() v1alpha1.IfaceSelector {
	return v1alpha1.IfaceByName("eth0")
}

func detectWSL() bool {
	// "Official" way of detecting WSL https://github.com/Microsoft/WSL/issues/423#issuecomment-221627364
	contents, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err == nil && (bytes.Contains(bytes.ToLower(contents), []byte("microsoft")) || bytes.Contains(bytes.ToLower(contents), []byte("wsl"))) {
		return true
	}

	return false
}
