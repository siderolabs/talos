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

	"github.com/siderolabs/talos/internal/pkg/tui/components"
	"github.com/siderolabs/talos/pkg/images"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// NewState creates new installer state.
//
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
		opts.ClusterConfig.ControlPlane.Endpoint = fmt.Sprintf("https://%s", nethelpers.JoinHostPort(conn.bootstrapEndpoint, constants.DefaultControlPlanePort))
	} else {
		opts.ClusterConfig.ControlPlane.Endpoint = fmt.Sprintf("https://%s", nethelpers.JoinHostPort(conn.nodeEndpoint, constants.DefaultControlPlanePort))
	}

	installDiskOptions := []any{
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

	var machineTypes []any

	if conn.ExpandingCluster() {
		machineTypes = []any{
			" worker ", machineapi.MachineConfig_MachineType(machine.TypeWorker),
			" control plane ", machineapi.MachineConfig_MachineType(machine.TypeControlPlane),
		}
		opts.MachineConfig.Type = machineapi.MachineConfig_MachineType(machine.TypeControlPlane)
	} else {
		machineTypes = []any{
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
			describe[v1alpha1.NetworkConfig]("hostname", true),
			&opts.MachineConfig.NetworkConfig.Hostname,
		),
		components.NewItem(
			"DNS Domain",
			describe[v1alpha1.ClusterNetworkConfig]("dnsDomain", true),
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
			components.NewSeparator(describe[v1alpha1.ClusterNetworkConfig]("cni", true)),
			components.NewItem(
				"Type",
				describe[v1alpha1.ClusterNetworkConfig]("cni", true),
				&state.cni,
				components.NewTableHeaders("CNI", "description"),
				constants.FlannelCNI, "CNI used by Talos by default",
				constants.NoneCNI, "CNI will not be installed",
			))
	}

	state.pages = []*Page{
		NewPage("Installer Params",
			components.NewItem(
				"Image",
				describe[v1alpha1.InstallConfig]("image", true),
				&opts.MachineConfig.InstallConfig.InstallImage,
			),
			components.NewSeparator(
				describe[v1alpha1.InstallConfig]("disk", true),
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
				describe[v1alpha1.MachineConfig]("type", true),
				&opts.MachineConfig.Type,
				machineTypes...,
			),
			components.NewItem(
				"Cluster Name",
				describe[v1alpha1.ClusterConfig]("clusterName", true),
				&opts.ClusterConfig.Name,
			),
			components.NewItem(
				"Control Plane Endpoint",
				describe[v1alpha1.ControlPlaneConfig]("endpoint", true),
				&opts.ClusterConfig.ControlPlane.Endpoint,
			),
			components.NewItem(
				"Kubernetes Version",
				"",
				&opts.MachineConfig.KubernetesVersion,
			),
			components.NewItem(
				"Allow Scheduling on Control Planes",
				describe[v1alpha1.ClusterConfig]("allowSchedulingOnControlPlanes", true),
				&opts.ClusterConfig.AllowSchedulingOnControlPlanes,
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
						"Indicates if DHCP should be used to configure the interface.",
						&adapterSettings.Dhcp,
					),
					components.NewItem(
						"Ignore",
						"Indicates if the interface should be ignored (skips configuration).",
						&adapterSettings.Ignore,
					),
					components.NewItem(
						"CIDR",
						"Assigns static IP addresses to the interface.",
						&adapterSettings.Cidr,
					),
					components.NewItem(
						"MTU",
						"Maximum Transmission Unit for the interface.",
						&adapterSettings.Mtu,
					),
					components.NewItem(
						"Route Metric",
						"Sets the priority of all routes assigned to this interface.",
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

type documentable interface {
	Doc() *encoder.Doc
}

func describe[T documentable](field string, short bool) string {
	var zeroT T

	return zeroT.Doc().Describe(field, short)
}
