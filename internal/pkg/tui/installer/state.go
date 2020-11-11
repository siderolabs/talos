// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package installer contains terminal UI based talos interactive installer parts.
package installer

import (
	"context"
	"fmt"

	"github.com/dustin/go-humanize"

	"github.com/talos-systems/talos/pkg/images"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// NewState creates new installer state.
func NewState(ctx context.Context, endpoint string, c *client.Client) (*State, error) {
	opts := &machineapi.GenerateConfigurationRequest{
		ConfigVersion: "v1alpha1",
		MachineConfig: &machineapi.MachineConfig{
			Type:              machineapi.MachineConfig_MachineType(machine.TypeInit),
			NetworkConfig:     &machineapi.NetworkConfig{},
			KubernetesVersion: constants.DefaultKubernetesVersion,
			InstallConfig: &machineapi.InstallConfig{
				InstallImage: images.DefaultInstallerImage,
			},
		},
		ClusterConfig: &machineapi.ClusterConfig{
			Name: "talos-default",
			ControlPlane: &machineapi.ControlPlaneConfig{
				Endpoint: fmt.Sprintf("https://%s:%d", endpoint, constants.DefaultControlPlanePort),
			},
			ClusterNetwork: &machineapi.ClusterNetworkConfig{
				DnsDomain: "cluster.local",
			},
		},
	}

	diskInstallOptions := []interface{}{}

	disks, err := c.Disks(ctx)
	if err != nil {
		return nil, err
	}

	for i, disk := range disks.Disks {
		if i == 0 {
			opts.MachineConfig.InstallConfig.InstallDisk = disk.DeviceName
		}

		name := fmt.Sprintf("%s  %s  %s", disk.DeviceName, disk.Model, humanize.Bytes(disk.Size))
		diskInstallOptions = append(diskInstallOptions, name, disk.DeviceName)
	}

	state := &State{
		client: c,
		opts:   opts,
		pages: []*Page{
			NewPage("Machine Config",
				NewItem(
					"cluster name",
					v1alpha1.ClusterConfigDoc.Describe("clusterName", true),
					&opts.ClusterConfig.Name,
				),
				NewItem(
					"machine type",
					v1alpha1.MachineConfigDoc.Describe("type", true),
					&opts.MachineConfig.Type,
					"init", machineapi.MachineConfig_MachineType(machine.TypeInit), // TODO: add more machine types when supported
					"controlplane", machineapi.MachineConfig_MachineType(machine.TypeControlPlane),
				),
				NewItem(
					"kubernetes version",
					"Kubernetes version to install.",
					&opts.MachineConfig.KubernetesVersion,
				),
				NewItem(
					"install disk",
					v1alpha1.InstallConfigDoc.Describe("disk", true),
					&opts.MachineConfig.InstallConfig.InstallDisk,
					diskInstallOptions...,
				),
				NewItem(
					"image",
					v1alpha1.InstallConfigDoc.Describe("image", true),
					&opts.MachineConfig.InstallConfig.InstallImage,
				),
			),
			NewPage("Network Config",
				NewItem(
					"hostname",
					v1alpha1.NetworkConfigDoc.Describe("hostname", true),
					&opts.MachineConfig.NetworkConfig.Hostname,
				),
				NewItem(
					"dns domain",
					v1alpha1.ClusterNetworkConfigDoc.Describe("dnsDomain", true),
					&opts.ClusterConfig.ClusterNetwork.DnsDomain,
				),
			),
		},
	}

	return state, nil
}

// State installer state.
type State struct {
	pages  []*Page
	opts   *machineapi.GenerateConfigurationRequest
	client *client.Client
}

// GenConfig returns current config encoded in yaml.
func (s *State) GenConfig(ctx context.Context) (*machineapi.GenerateConfigurationResponse, error) {
	return s.client.GenerateConfiguration(ctx, s.opts)
}
