// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configuration

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
	v1alpha1machine "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Generate config for GenerateConfiguration grpc.
//
//nolint:gocyclo,cyclop
func Generate(ctx context.Context, in *machine.GenerateConfigurationRequest) (reply *machine.GenerateConfigurationResponse, err error) {
	var c config.Provider

	if in.MachineConfig == nil || in.ClusterConfig == nil || in.ClusterConfig.ControlPlane == nil {
		return nil, fmt.Errorf("invalid generate request")
	}

	switch in.ConfigVersion {
	case "v1alpha1":
		machineType := v1alpha1machine.Type(in.MachineConfig.Type)

		options := []generate.GenOption{}

		if in.MachineConfig.NetworkConfig != nil {
			networkConfig := &v1alpha1.NetworkConfig{
				NetworkHostname: in.MachineConfig.NetworkConfig.Hostname,
			}

			networkInterfaces := in.MachineConfig.NetworkConfig.Interfaces
			if len(networkInterfaces) > 0 {
				networkConfig.NetworkInterfaces = make([]*v1alpha1.Device, len(networkInterfaces))

				for i, device := range networkInterfaces {
					routes := make([]*v1alpha1.Route, len(device.Routes))

					iface := &v1alpha1.Device{
						DeviceInterface: device.Interface,
						DeviceMTU:       int(device.Mtu),
						DeviceCIDR:      device.Cidr,
						DeviceDHCP:      device.Dhcp,
						DeviceIgnore:    device.Ignore,
						DeviceRoutes:    routes,
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

			options = append(options, generate.WithAllowSchedulingOnMasters(in.ClusterConfig.AllowSchedulingOnMasters))
		}

		var (
			input         *generate.Input
			cfgBytes      []byte
			taloscfgBytes []byte
			baseConfig    config.Provider
			secrets       *generate.SecretsBundle
		)

		baseConfig, err = configloader.NewFromFile(constants.ConfigPath)

		clock := generate.NewClock()

		if in.OverrideTime != nil {
			clock.SetFixedTimestamp(in.OverrideTime.AsTime())
		}

		switch {
		case os.IsNotExist(err):
			secrets, err = generate.NewSecretsBundle(clock)
			if err != nil {
				return nil, err
			}
		case err != nil:
			return nil, err
		default:
			secrets = generate.NewSecretsBundleFromConfig(clock, baseConfig)
		}

		input, err = generate.NewInput(
			in.ClusterConfig.Name,
			in.ClusterConfig.ControlPlane.Endpoint,
			in.MachineConfig.KubernetesVersion,
			secrets,
			options...,
		)

		if err != nil {
			return nil, err
		}

		c, err = generate.Config(
			machineType,
			input,
		)

		if err != nil {
			return nil, err
		}

		cfgBytes, err = c.Bytes()

		if err != nil {
			return nil, err
		}

		talosconfig, err := generate.Talosconfig(input, options...)
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
