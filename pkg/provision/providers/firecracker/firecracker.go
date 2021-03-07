// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package firecracker implements Provisioner via Firecracker VMs.
package firecracker

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

// NewProvisioner initializes firecracker provisioner.
func NewProvisioner(ctx context.Context) (provision.Provisioner, error) {
	p := &provisioner{
		vm.Provisioner{
			Name: "firecracker",
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
	nameservers := make([]string, len(networkReq.Nameservers))
	for i := range nameservers {
		nameservers[i] = networkReq.Nameservers[i].String()
	}

	return []generate.GenOption{
		generate.WithInstallDisk("/dev/vda"),
		generate.WithInstallExtraKernelArgs([]string{
			"console=ttyS0",
			// reboot configuration
			"reboot=k",
			"panic=1",
			// disable stuff we don't need
			"pci=off",
			"acpi=off",
			"i8042.noaux=",
			// Talos-specific
			"talos.platform=metal",
		}),
		generate.WithNetworkOptions(
			v1alpha1.WithNetworkNameservers(nameservers...),
			v1alpha1.WithNetworkInterfaceCIDR("eth0", "169.254.128.128/32"), // link-local IP just to trigger the static networkd config
			v1alpha1.WithNetworkInterfaceMTU("eth0", networkReq.MTU),
		),
	}
}

// GetLoadBalancers returns internal/external loadbalancer endpoints.
func (p *provisioner) GetLoadBalancers(networkReq provision.NetworkRequest) (internalEndpoint, externalEndpoint string) {
	// firecracker runs loadbalancer on the bridge, which is good for both internal access, external access goes via round-robin
	return networkReq.GatewayAddrs[0].String(), ""
}

// GetFirstInterface returns first network interface name.
func (p *provisioner) GetFirstInterface() string {
	return "eth0"
}
