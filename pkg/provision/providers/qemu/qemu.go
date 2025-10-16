// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
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
func (p *provisioner) GenOptions(networkReq provision.NetworkRequest, contract *config.VersionContract) ([]generate.Option, []bundle.Option) {
	hasIPv4 := false
	hasIPv6 := false

	for _, cidr := range networkReq.CIDRs {
		if cidr.Addr().Is6() {
			hasIPv6 = true
		} else {
			hasIPv4 = true
		}
	}

	genOpts := []generate.Option{
		generate.WithInstallDisk("/dev/vda"),
	}

	var bundleOpts []bundle.Option

	if contract.MultidocNetworkConfigSupported() {
		aliasConfig := networkcfg.NewLinkAliasConfigV1Alpha1("net0")
		aliasConfig.Selector = networkcfg.LinkSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`link.driver == "virtio_net"`, celenv.LinkLocator())),
		}

		documents := []configconfig.Document{aliasConfig}

		if hasIPv4 {
			dhcp4Config := networkcfg.NewDHCPv4ConfigV1Alpha1("net0")
			documents = append(documents, dhcp4Config)
		} else if hasIPv6 {
			dhcp6Config := networkcfg.NewDHCPv6ConfigV1Alpha1("net0")
			documents = append(documents, dhcp6Config)
		}

		ctr, err := container.New(documents...)
		if err != nil {
			panic(err)
		}

		bundleOpts = append(bundleOpts,
			bundle.WithPatch([]configpatcher.Patch{configpatcher.NewStrategicMergePatch(ctr)}),
		)
	} else {
		virtioSelector := v1alpha1.IfaceBySelector(v1alpha1.NetworkDeviceSelector{
			NetworkDeviceKernelDriver: "virtio_net",
		})

		genOpts = append(genOpts,
			generate.WithNetworkOptions(
				v1alpha1.WithNetworkInterfaceDHCP(virtioSelector, true),
				v1alpha1.WithNetworkInterfaceDHCPv4(virtioSelector, hasIPv4),
				v1alpha1.WithNetworkInterfaceDHCPv6(virtioSelector, hasIPv6),
			),
		)
	}

	if !contract.GrubUseUKICmdlineDefault() {
		genOpts = append(genOpts,
			generate.WithInstallExtraKernelArgs([]string{
				"console=ttyS0", // TODO: should depend on arch
				// reboot configuration
				"reboot=k",
				"panic=1",
				"talos.shutdown=halt",
				// Talos-specific
				"talos.platform=metal",
			}),
		)
	}

	return genOpts, bundleOpts
}

// GetInClusterKubernetesControlPlaneEndpoint returns the Kubernetes control plane endpoint.
func (p *provisioner) GetInClusterKubernetesControlPlaneEndpoint(networkReq provision.NetworkRequest, controlPlanePort int) string {
	// QEMU provisioner always runs TCP loadbalancer on the bridge IP and port 6443.
	return "https://" + nethelpers.JoinHostPort(networkReq.GatewayAddrs[0].String(), controlPlanePort)
}

// GetExternalKubernetesControlPlaneEndpoint returns the Kubernetes control plane endpoint.
func (p *provisioner) GetExternalKubernetesControlPlaneEndpoint(networkReq provision.NetworkRequest, controlPlanePort int) string {
	// for QEMU, external and in-cluster endpoints are same.
	return p.GetInClusterKubernetesControlPlaneEndpoint(networkReq, controlPlanePort)
}

// GetTalosAPIEndpoints returns a list of Talos API endpoints.
func (p *provisioner) GetTalosAPIEndpoints(provision.NetworkRequest) []string {
	// nil means that the API of controlplane endpoints should be used
	return nil
}

// GetFirstInterface returns first network interface name.
func (p *provisioner) GetFirstInterface() v1alpha1.IfaceSelector {
	return v1alpha1.IfaceBySelector(v1alpha1.NetworkDeviceSelector{
		NetworkDeviceKernelDriver: "virtio_net",
	})
}
