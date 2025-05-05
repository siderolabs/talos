// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package configuration implements configuration generation.
package configuration

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	v1alpha1machine "github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	machineconfig "github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// Generate config for GenerateConfiguration grpc.
//
//nolint:gocyclo,cyclop
func Generate(ctx context.Context, st state.State, in *machine.GenerateConfigurationRequest) (reply *machine.GenerateConfigurationResponse, err error) {
	var c config.Provider

	if in.MachineConfig == nil || in.ClusterConfig == nil || in.ClusterConfig.ControlPlane == nil {
		return nil, errors.New("invalid generate request")
	}

	switch in.ConfigVersion {
	case "v1alpha1":
		machineType := v1alpha1machine.Type(in.MachineConfig.Type)

		var options []generate.Option

		if in.MachineConfig.NetworkConfig != nil {
			networkConfig := &v1alpha1.NetworkConfig{
				NetworkHostname: in.MachineConfig.NetworkConfig.Hostname,
			}

			networkInterfaces := in.MachineConfig.NetworkConfig.Interfaces
			if len(networkInterfaces) > 0 {
				networkConfig.NetworkInterfaces = make([]*v1alpha1.Device, len(networkInterfaces))

				for i, device := range networkInterfaces {
					iface := &v1alpha1.Device{
						DeviceInterface: device.Interface,
						DeviceMTU:       int(device.Mtu),
						DeviceCIDR:      device.Cidr,
						DeviceDHCP:      pointer.To(device.Dhcp),
						DeviceIgnore:    pointer.To(device.Ignore),
						DeviceRoutes: xslices.Map(device.Routes, func(route *machine.RouteConfig) *v1alpha1.Route {
							return &v1alpha1.Route{
								RouteNetwork: route.Network,
								RouteGateway: route.Gateway,
								RouteMetric:  route.Metric,
							}
						}),
					}

					if device.DhcpOptions != nil {
						iface.DeviceDHCPOptions = &v1alpha1.DHCPOptions{
							DHCPRouteMetric: device.DhcpOptions.RouteMetric,
						}
					}

					networkConfig.NetworkInterfaces[i] = iface
				}
			}

			options = append(options, generate.WithNetworkOptions(v1alpha1.WithNetworkConfig(networkConfig)))
		}

		if in.MachineConfig.InstallConfig != nil {
			if in.MachineConfig.InstallConfig.InstallDisk != "" {
				options = append(options, generate.WithInstallDisk(in.MachineConfig.InstallConfig.InstallDisk))
			}

			if in.MachineConfig.InstallConfig.InstallImage != "" {
				options = append(options, generate.WithInstallImage(in.MachineConfig.InstallConfig.InstallImage))
			}
		}

		if in.ClusterConfig.ClusterNetwork != nil {
			if in.ClusterConfig.ClusterNetwork.DnsDomain != "" {
				options = append(options, generate.WithDNSDomain(in.ClusterConfig.ClusterNetwork.DnsDomain))
			}

			if in.ClusterConfig.ClusterNetwork.CniConfig != nil {
				options = append(options, generate.WithClusterCNIConfig(&v1alpha1.CNIConfig{
					CNIName: in.ClusterConfig.ClusterNetwork.CniConfig.Name,
					CNIUrls: in.ClusterConfig.ClusterNetwork.CniConfig.Urls,
				}))
			}
		}

		options = append(options, generate.WithAllowSchedulingOnControlPlanes(in.ClusterConfig.AllowSchedulingOnControlPlanes))

		var (
			input         *generate.Input
			cfgBytes      []byte
			taloscfgBytes []byte
			baseConfig    config.Provider
			secretsBundle *secrets.Bundle
		)

		cfgResource, err := safe.StateGetByID[*machineconfig.MachineConfig](ctx, st, machineconfig.ActiveID)
		if cfgResource != nil {
			baseConfig = cfgResource.Provider()
		}

		clock := secrets.NewFixedClock(time.Now())

		if in.OverrideTime != nil {
			clock = secrets.NewFixedClock(in.OverrideTime.AsTime())
		}

		switch {
		case state.IsNotFoundError(err):
			secretsBundle, err = secrets.NewBundle(clock, config.TalosVersionCurrent)
			if err != nil {
				return nil, err
			}
		case err != nil:
			return nil, err
		default:
			secretsBundle = secrets.NewBundleFromConfig(clock, baseConfig)
		}

		options = append(options, generate.WithSecretsBundle(secretsBundle))

		input, err = generate.NewInput(
			in.ClusterConfig.Name,
			in.ClusterConfig.ControlPlane.Endpoint,
			in.MachineConfig.KubernetesVersion,
			options...,
		)
		if err != nil {
			return nil, err
		}

		c, err = input.Config(machineType)
		if err != nil {
			return nil, err
		}

		cfgBytes, err = c.Bytes()
		if err != nil {
			return nil, err
		}

		talosconfig, err := input.Talosconfig()
		if err != nil {
			return nil, err
		}

		endpoint, err := url.Parse(in.ClusterConfig.ControlPlane.Endpoint)
		if err != nil {
			return nil, err
		}

		talosconfig.Contexts[talosconfig.Context].Endpoints = []string{
			endpoint.Hostname(),
		}

		taloscfgBytes, err = talosconfig.Bytes()
		if err != nil {
			return nil, err
		}

		reply = &machine.GenerateConfigurationResponse{
			Messages: []*machine.GenerateConfiguration{
				{
					Data:        [][]byte{cfgBytes},
					Talosconfig: taloscfgBytes,
				},
			},
		}
	default:
		return nil, fmt.Errorf("unsupported config version %s", in.ConfigVersion)
	}

	return reply, nil
}
