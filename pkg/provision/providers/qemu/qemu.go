// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"

	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

// Provisioner is the qemu provisioner.
type Provisioner struct {
	vm.Provisioner
}

// NewQemuProvisioner initializes a new (non generic) qemu provisioner.
func NewQemuProvisioner(ctx context.Context) (Provisioner, error) {
	p := Provisioner{
		Provisioner: vm.Provisioner{
			Name: "qemu",
		},
	}

	return p, nil
}

// NewProvisioner initializes a new generic qemu provisioner.
func NewProvisioner(ctx context.Context) (provision.Provisioner, error) {
	p := &Provisioner{
		vm.Provisioner{
			Name: "qemu",
		},
	}

	return p, nil
}

// Close and release resources.
func (p *Provisioner) Close() error {
	return nil
}

// GenOptions provides a list of additional config generate options.
func (p *Provisioner) GenOptions(networkReq provision.NetworkRequestBase) []generate.Option {
	hasIPv4 := false
	hasIPv6 := false

	for _, cidr := range networkReq.CIDRs {
		if cidr.Addr().Is6() {
			hasIPv6 = true
		} else {
			hasIPv4 = true
		}
	}

	virtioSelector := v1alpha1.IfaceBySelector(v1alpha1.NetworkDeviceSelector{
		NetworkDeviceKernelDriver: "virtio_net",
	})

	return []generate.Option{
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
			v1alpha1.WithNetworkInterfaceDHCP(virtioSelector, true),
			v1alpha1.WithNetworkInterfaceDHCPv4(virtioSelector, hasIPv4),
			v1alpha1.WithNetworkInterfaceDHCPv6(virtioSelector, hasIPv6),
		),
	}
}

// GetInClusterKubernetesControlPlaneEndpoint returns the Kubernetes control plane endpoint.
func (p *Provisioner) GetInClusterKubernetesControlPlaneEndpoint(networkReq provision.NetworkRequestBase, controlPlanePort int) string {
	// QEMU provisioner always runs TCP loadbalancer on the bridge IP and port 6443.
	return "https://" + nethelpers.JoinHostPort(networkReq.GatewayAddrs[0].String(), controlPlanePort)
}

// GetExternalKubernetesControlPlaneEndpoint returns the Kubernetes control plane endpoint.
func (p *Provisioner) GetExternalKubernetesControlPlaneEndpoint(networkReq provision.NetworkRequestBase, controlPlanePort int) string {
	// for QEMU, external and in-cluster endpoints are same.
	return p.GetInClusterKubernetesControlPlaneEndpoint(networkReq, controlPlanePort)
}

// GetTalosAPIEndpoints returns a list of Talos API endpoints.
func (p *Provisioner) GetTalosAPIEndpoints(provision.NetworkRequestBase) []string {
	// nil means that the API of controlplane endpoints should be used
	return nil
}

// GetFirstInterface returns first network interface name.
func (p *Provisioner) GetFirstInterface() v1alpha1.IfaceSelector {
	return v1alpha1.IfaceBySelector(v1alpha1.NetworkDeviceSelector{
		NetworkDeviceKernelDriver: "virtio_net",
	})
}
