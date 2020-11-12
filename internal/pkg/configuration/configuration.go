// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configuration

import (
	"context"
	"fmt"

	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
	v1alpha1machine "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// Generate config for GenerateConfiguration grpc.
//
// nolint:gocyclo
func Generate(ctx context.Context, in *machine.GenerateConfigurationRequest) (reply *machine.GenerateConfigurationResponse, err error) {
	var config config.Provider

	if in.MachineConfig == nil || in.ClusterConfig == nil || in.ClusterConfig.ControlPlane == nil {
		return nil, fmt.Errorf("invalid generate request")
	}

	switch in.ConfigVersion {
	case "v1alpha1":
		machineType := v1alpha1machine.Type(in.MachineConfig.Type)

		options := []generate.GenOption{}

		if in.MachineConfig.NetworkConfig != nil {
			generate.WithNetworkConfig(
				&v1alpha1.NetworkConfig{
					NetworkHostname: in.MachineConfig.NetworkConfig.Hostname,
				},
			)
		}

		if in.MachineConfig.InstallConfig != nil {
			if in.MachineConfig.InstallConfig.InstallDisk != "" {
				generate.WithInstallDisk(in.MachineConfig.InstallConfig.InstallDisk)
			}

			if in.MachineConfig.InstallConfig.InstallImage != "" {
				generate.WithInstallImage(in.MachineConfig.InstallConfig.InstallImage)
			}
		}

		if in.ClusterConfig.ClusterNetwork != nil {
			if in.ClusterConfig.ClusterNetwork.DnsDomain != "" {
				generate.WithDNSDomain(in.ClusterConfig.ClusterNetwork.DnsDomain)
			}
		}

		var (
			input    *generate.Input
			cfgBytes []byte
		)

		input, err = generate.NewInput(
			in.ClusterConfig.Name,
			in.ClusterConfig.ControlPlane.Endpoint,
			in.MachineConfig.KubernetesVersion,
			options...,
		)

		if err != nil {
			return nil, err
		}

		config, err = generate.Config(
			machineType,
			input,
		)

		if err != nil {
			return nil, err
		}

		cfgBytes, err = config.Bytes()

		if err != nil {
			return nil, err
		}

		reply.Data = [][]byte{cfgBytes}
	default:
		return nil, fmt.Errorf("unsupported config version %s", in.ConfigVersion)
	}

	return reply, nil
}
