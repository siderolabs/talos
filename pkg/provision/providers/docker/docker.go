// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package docker implements Provisioner via docker.
package docker

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/docker/docker/client"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/provision"
)

type provisioner struct {
	client *client.Client

	mappedKubernetesPort, mappedTalosAPIPort int
}

func getAvailableTCPPort(ctx context.Context) (int, error) {
	l, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	_, portStr, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		l.Close() //nolint:errcheck

		return 0, err
	}

	err = l.Close()
	if err != nil {
		return 0, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, err
	}

	return port, nil
}

// NewProvisioner initializes docker provisioner.
func NewProvisioner(ctx context.Context) (provision.Provisioner, error) {
	p := &provisioner{}

	var err error

	p.client, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	p.mappedKubernetesPort, err = getAvailableTCPPort(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get available port for Kubernetes API: %w", err)
	}

	p.mappedTalosAPIPort, err = getAvailableTCPPort(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get available port for Talos API: %w", err)
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
func (p *provisioner) GenOptions(networkReq provision.NetworkRequest, _ *config.VersionContract) ([]generate.Option, []bundle.Option) {
	return []generate.Option{
		generate.WithNetworkOptions(
			v1alpha1.WithNetworkInterfaceIgnore(v1alpha1.IfaceByName("eth0")),
		),
		generate.WithHostDNSForwardKubeDNSToHost(true),
	}, nil
}

// GetInClusterKubernetesControlPlaneEndpoint returns the Kubernetes control plane endpoint.
func (p *provisioner) GetInClusterKubernetesControlPlaneEndpoint(networkReq provision.NetworkRequest, controlPlanePort int) string {
	// Docker doesn't have a loadbalancer, so use the first container IP.
	return "https://" + nethelpers.JoinHostPort(networkReq.CIDRs[0].Addr().Next().Next().String(), controlPlanePort)
}

// GetExternalKubernetesControlPlaneEndpoint returns the Kubernetes control plane endpoint.
func (p *provisioner) GetExternalKubernetesControlPlaneEndpoint(provision.NetworkRequest, int) string {
	// return a mapped to the localhost first container Kubernetes API endpoint.
	return "https://" + nethelpers.JoinHostPort("127.0.0.1", p.mappedKubernetesPort)
}

// GetTalosAPIEndpoints returns a list of Talos API endpoints.
func (p *provisioner) GetTalosAPIEndpoints(provision.NetworkRequest) []string {
	// return a mapped to the localhost first container Talos API endpoint.
	return []string{nethelpers.JoinHostPort("127.0.0.1", p.mappedTalosAPIPort)}
}

// UserDiskName not implemented for docker.
func (p *provisioner) UserDiskName(index int) string {
	return ""
}

// GetFirstInterface returns first network interface name.
func (p *provisioner) GetFirstInterface() v1alpha1.IfaceSelector {
	return v1alpha1.IfaceByName("eth0")
}
