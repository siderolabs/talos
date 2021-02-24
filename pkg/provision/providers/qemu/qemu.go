// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/provision"
	"github.com/talos-systems/talos/pkg/provision/providers/vm"
)

type provisioner struct {
	vm.Provisioner
}

// NewProvisioner initializes qemu provisioner.
func NewProvisioner(ctx context.Context) (provision.Provisioner, error) {
	p := &provisioner{
		vm.Provisioner{
			Name: "qemu",
		},
	}

	return p, nil
}

// Close and release resources.
func (p *provisioner) Close() error {
	return nil
}

// GenOptions provides a list of additional config generate options.
func (p *provisioner) GenOptions(networkReq provision.NetworkRequest) []generate.GenOption {
	hasIPv4 := false
	hasIPv6 := false

	for _, cidr := range networkReq.CIDRs {
		if cidr.IP.To4() == nil {
			hasIPv6 = true
		} else {
			hasIPv4 = true
		}
	}

	return []generate.GenOption{
		generate.WithInstallDisk("/dev/vda"),
		generate.WithInstallExtraKernelArgs([]string{
			"console=ttyS0", // TODO: should depend on arch
			// reboot configuration
			"reboot=k",
			"panic=1",
			"talos.shutdown=halt",
			// Talos-specific
			"talos.platform=metal",
		}),
		generate.WithNetworkOptions(
			v1alpha1.WithNetworkInterfaceDHCP("eth0", true),
			v1alpha1.WithNetworkInterfaceDHCPv4("eth0", hasIPv4),
			v1alpha1.WithNetworkInterfaceDHCPv6("eth0", hasIPv6),
		),
	}
}

// GetLoadBalancers returns internal/external loadbalancer endpoints.
func (p *provisioner) GetLoadBalancers(networkReq provision.NetworkRequest) (internalEndpoint, externalEndpoint string) {
	// qemu runs loadbalancer on the bridge, which is good for both internal access, external access goes via round-robin
	return networkReq.GatewayAddrs[0].String(), ""
}

// GetFirstInterface returns first network interface name.
func (p *provisioner) GetFirstInterface() string {
	return "eth0"
}
