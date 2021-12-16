// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package installer contains terminal UI based talos interactive installer parts.
package installer

import (
	"context"
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/rivo/tview"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/talos-systems/talos/internal/pkg/tui/components"
	"github.com/talos-systems/talos/pkg/images"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

const canalCustomCNI = "canal"

var customCNIPresets = map[string][]string{
	canalCustomCNI: {
		"https://docs.projectcalico.org/archive/v3.20/manifests/canal.yaml",
	},
}

// NewState creates new installer state.
//nolint:gocyclo
func NewState(ctx context.Context, installer *Installer, conn *Connection) (*State, error) {
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
			Name:         "talos-default",
			ControlPlane: &machineapi.ControlPlaneConfig{},
			ClusterNetwork: &machineapi.ClusterNetworkConfig{
				DnsDomain: "cluster.local",
				CniConfig: nil, // set at GenConfig
			},
		},
	}

	if conn.ExpandingCluster() {
		opts.ClusterConfig.ControlPlane.Endpoint = fmt.Sprintf("https://%s:%d", conn.bootstrapEndpoint, constants.DefaultControlPlanePort)
	} else {
		opts.ClusterConfig.ControlPlane.Endpoint = fmt.Sprintf("https://%s:%d", conn.nodeEndpoint, constants.DefaultControlPlanePort)
	}

	installDiskOptions := []interface{}{
		components.NewTableHeaders("DEVICE NAME", "MODEL NAME", "SIZE"),
	}

	disks, err := conn.Disks()
	if err != nil {
		return nil, err
	}

	for _, msg := range disks.Messages {
		for i, disk := range msg.Disks {
			if i == 0 {
				opts.MachineConfig.InstallConfig.InstallDisk = disk.DeviceName
			}

			installDiskOptions = append(installDiskOptions, disk.DeviceName, disk.Model, humanize.Bytes(disk.Size))
		}
	}

	var machineTypes []interface{}

	if conn.ExpandingCluster() {
		machineTypes = []interface{}{
			" worker ", machineapi.MachineConfig_MachineType(machine.TypeWorker),
			" control plane ", machineapi.MachineConfig_MachineType(machine.TypeControlPlane),
		}
		opts.MachineConfig.Type = machineapi.MachineConfig_MachineType(machine.TypeControlPlane)
	} else {
		machineTypes = []interface{}{
			" control plane ", machineapi.MachineConfig_MachineType(machine.TypeInit),
		}
	}

	state := &State{
		opts: opts,
		conn: conn,
		cni:  constants.FlannelCNI,
	}

	networkConfigItems := []*components.Item{
		components.NewItem(
			"Hostname",
			v1alpha1.NetworkConfigDoc.Describe("hostname", true),
			&opts.MachineConfig.NetworkConfig.Hostname,
		),
		components.NewItem(
			"DNS Domain",
			v1alpha1.ClusterNetworkConfigDoc.Describe("dnsDomain", true),
			&opts.ClusterConfig.ClusterNetwork.DnsDomain,
		),
	}

	links, err := conn.Links()
	if err != nil {
		return nil, err
	}

	addedInterfaces := false
	opts.MachineConfig.NetworkConfig.Interfaces = []*machineapi.NetworkDeviceConfig{}

	for _, link := range links {
		link := link

		status := ""

		if !link.Physical {
			continue
		}

		if link.Up {
			status = " (UP)"
		}

		if !addedInterfaces {
			networkConfigItems = append(networkConfigItems, components.NewSeparator("Network Interfaces Configuration"))
			addedInterfaces = true
		}

		networkConfigItems = append(networkConfigItems, components.NewItem(
			fmt.Sprintf("%s, %s%s", link.Name, link.HardwareAddr, status),
			"",
			configureAdapter(installer, opts, &link),
		))
	}

	if !conn.ExpandingCluster() {
		networkConfigItems = append(networkConfigItems,
			components.NewSeparator(v1alpha1.ClusterNetworkConfigDoc.Describe("cni", true)),
			components.NewItem(
				"Type",
				v1alpha1.ClusterNetworkConfigDoc.Describe("cni", true),
				&state.cni,
				components.NewTableHeaders("CNI", "description"),
				constants.FlannelCNI, "CNI used by Talos by default",
				canalCustomCNI, "Canal v3.20",
				constants.NoneCNI, "CNI will not be installed",
			))
	}

	state.pages = []*Page{
		NewPage("Installer Params",
			components.NewItem(
				"Image",
				v1alpha1.InstallConfigDoc.Describe("image", true),
				&opts.MachineConfig.InstallConfig.InstallImage,
			),
			components.NewSeparator(
				v1alpha1.InstallConfigDoc.Describe("disk", true),
			),
			components.NewItem(
				"Install Disk",
				"",
				&opts.MachineConfig.InstallConfig.InstallDisk,
				installDiskOptions...,
			),
		),
		NewPage("Machine Config",
			components.NewItem(
				"Machine Type",
				v1alpha1.MachineConfigDoc.Describe("type", true),
				&opts.MachineConfig.Type,
				machineTypes...,
			),
			components.NewItem(
				"Cluster Name",
				v1alpha1.ClusterConfigDoc.Describe("clusterName", true),
				&opts.ClusterConfig.Name,
			),
			components.NewItem(
				"Control Plane Endpoint",
				v1alpha1.ControlPlaneConfigDoc.Describe("endpoint", true),
				&opts.ClusterConfig.ControlPlane.Endpoint,
			),
			components.NewItem(
				"Kubernetes Version",
				"",
				&opts.MachineConfig.KubernetesVersion,
			),
			components.NewItem(
				"Allow Scheduling on Masters",
				v1alpha1.ClusterConfigDoc.Describe("allowSchedulingOnMasters", true),
				&opts.ClusterConfig.AllowSchedulingOnMasters,
			),
		),
		NewPage("Network Config",
			networkConfigItems...,
		),
	}

	return state, nil
}

// State installer state.
type State struct {
	pages []*Page
	opts  *machineapi.GenerateConfigurationRequest
	conn  *Connection
	cni   string
}

// GenConfig returns current config encoded in yaml.
func (s *State) GenConfig() (*machineapi.GenerateConfigurationResponse, error) {
	cniConfig := &machineapi.CNIConfig{
		Name: s.cni,
	}

	if urls, ok := customCNIPresets[s.cni]; ok {
		cniConfig.Name = constants.CustomCNI
		cniConfig.Urls = urls
	}

	s.opts.ClusterConfig.ClusterNetwork.CniConfig = cniConfig

	s.opts.OverrideTime = timestamppb.New(time.Now().UTC())

	return s.conn.GenerateConfiguration(s.opts)
}

func configureAdapter(installer *Installer, opts *machineapi.GenerateConfigurationRequest, link *Link) func(item *components.Item) tview.Primitive {
	return func(item *components.Item) tview.Primitive {
		return components.NewFormModalButton(item.Name, "configure").
			SetSelectedFunc(func() {
				deviceIndex := -1
				var adapterSettings *machineapi.NetworkDeviceConfig

				for i, iface := range opts.MachineConfig.NetworkConfig.Interfaces {
					if iface.Interface == link.Name {
						deviceIndex = i
						adapterSettings = iface

						break
					}
				}

				if adapterSettings == nil {
					adapterSettings = &machineapi.NetworkDeviceConfig{
						Interface:   link.Name,
						Dhcp:        true,
						Mtu:         int32(link.MTU),
						Ignore:      false,
						DhcpOptions: &machineapi.DHCPOptionsConfig{},
					}
				}

				items := []*components.Item{
					components.NewItem(
						"Use DHCP",
						v1alpha1.DeviceDoc.Describe("dhcp", true),
						&adapterSettings.Dhcp,
					),
					components.NewItem(
						"Ignore",
						v1alpha1.DeviceDoc.Describe("ignore", true),
						&adapterSettings.Ignore,
					),
					components.NewItem(
						"CIDR",
						v1alpha1.DeviceDoc.Describe("cidr", true),
						&adapterSettings.Cidr,
					),
					components.NewItem(
						"MTU",
						v1alpha1.DeviceDoc.Describe("mtu", true),
						&adapterSettings.Mtu,
					),
					components.NewItem(
						"Route Metric",
						v1alpha1.DeviceDoc.Describe("dhcpOptions", true),
						&adapterSettings.DhcpOptions.RouteMetric,
					),
				}

				adapterConfiguration := components.NewForm(installer.app)
				if err := adapterConfiguration.AddFormItems(items); err != nil {
					panic(err)
				}

				focused := installer.app.GetFocus()
				page, _ := installer.pages.GetFrontPage()

				goBack := func() {
					installer.pages.SwitchToPage(page)
					installer.app.SetFocus(focused)
				}

				adapterConfiguration.AddMenuButton("Cancel", false).SetSelectedFunc(func() {
					goBack()
				})

				adapterConfiguration.AddMenuButton("Apply", false).SetSelectedFunc(func() {
					goBack()

					if adapterSettings.Dhcp {
						adapterSettings.Cidr = ""
					}

					if deviceIndex == -1 {
						opts.MachineConfig.NetworkConfig.Interfaces = append(
							opts.MachineConfig.NetworkConfig.Interfaces,
							adapterSettings,
						)
					}
				})

				flex := tview.NewFlex().SetDirection(tview.FlexRow)
				flex.AddItem(tview.NewBox().SetBackgroundColor(color), 1, 0, false)
				flex.AddItem(adapterConfiguration, 0, 1, false)

				installer.addPage(
					fmt.Sprintf("Adapter %s Configuration", link.Name),
					flex,
					true,
					nil,
				)
				installer.app.SetFocus(adapterConfiguration)
			})
	}
}
