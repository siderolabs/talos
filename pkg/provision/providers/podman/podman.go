// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package podman implements Provisioner via podman.
package podman

import (
	"context"
	"os"
	"runtime"

	"github.com/containers/podman/v4/pkg/bindings"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/provision"
)

type provisioner struct {
	connection context.Context //nolint:containedctx
}

// NewProvisioner initializes podman provisioner.
func NewProvisioner(ctx context.Context) (provision.Provisioner, error) {
	p := &provisioner{}

	var err error

	p.connection, err = bindings.NewConnection(ctx, p.SocketURI())
	if err != nil {
		return nil, err
	}

	return p, nil
}

// Get podman API socket location.
func (p *provisioner) SocketURI() string {
	socket := os.Getenv("PODMAN_SOCKET")
	if socket != "" {
		return socket
	}

	sockDir := os.Getenv("XDG_RUNTIME_DIR")
	if sockDir == "" {
		sockDir = "/var/run"
	}

	return "unix:" + sockDir + "/podman/podman.sock"
}

// Close and release resources.
func (p *provisioner) Close() error {
	// Nothing to do
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
		generate.WithNetworkOptions(
			v1alpha1.WithNetworkInterfaceIgnore("eth0"),
			v1alpha1.WithNetworkNameservers(nameservers...),
		),
	}
}

// GetLoadBalancers returns internal/external loadbalancer endpoints.
func (p *provisioner) GetLoadBalancers(networkReq provision.NetworkRequest) (internalEndpoint, externalEndpoint string) {
	// podman doesn't provide internal LB, so return empty string
	// external LB is always localhost for OS X where docker exposes ports
	switch runtime.GOOS {
	case "darwin", "windows":
		return "", "127.0.0.1"
	default:
		return "", ""
	}
}

// UserDiskName not implemented for podman.
func (p *provisioner) UserDiskName(index int) string {
	return ""
}

// GetFirstInterface returns first network interface name.
func (p *provisioner) GetFirstInterface() string {
	return "eth0"
}
