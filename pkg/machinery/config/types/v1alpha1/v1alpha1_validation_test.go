// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

type runtimeMode struct {
	requiresInstall bool
}

func (m runtimeMode) String() string {
	return fmt.Sprintf("runtimeMode(%v)", m.requiresInstall)
}

func (m runtimeMode) RequiresInstall() bool {
	return m.requiresInstall
}

func TestValidate(t *testing.T) {
	t.Parallel()

	endpointURL, err := url.Parse("https://localhost:6443/")
	require.NoError(t, err)

	for _, test := range []struct {
		name             string
		config           *v1alpha1.Config
		requiresInstall  bool
		strict           bool
		expectedWarnings []string
		expectedError    string
	}{
		{
			name: "NoMachine",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
			},
			expectedError: "1 error occurred:\n\t* machine instructions are required\n\n",
		},
		{
			name: "NoMachineType",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedWarnings: []string{
				`use "worker" instead of "" for machine type`,
			},
		},
		{
			name: "JoinMachineType",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "join",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedWarnings: []string{
				`use "worker" instead of "join" for machine type`,
			},
		},
		{
			name: "NoMachineTypeStrict",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			strict:        true,
			expectedError: "1 error occurred:\n\t* warning: use \"worker\" instead of \"\" for machine type\n\n",
		},
		{
			name: "NoMachineInstall",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
		},
		{
			name: "NoMachineInstallRequired",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			requiresInstall: true,
			expectedError:   "1 error occurred:\n\t* install instructions are required in \"runtimeMode(true)\" mode\n\n",
		},
		{
			name: "MachineInstallDisk",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineInstall: &v1alpha1.InstallConfig{
						InstallDisk: "/dev/vda",
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			requiresInstall: true,
		},

		{
			name: "ExternalCloudProviderEnabled",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ExternalCloudProviderConfig: &v1alpha1.ExternalCloudProviderConfig{
						ExternalEnabled: true,
						ExternalManifests: []string{
							"https://www.example.com/manifest1.yaml",
							"https://www.example.com/manifest2.yaml",
						},
					},
				},
			},
		},
		{
			name: "ExternalCloudProviderEnabledNoManifests",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ExternalCloudProviderConfig: &v1alpha1.ExternalCloudProviderConfig{
						ExternalEnabled: true,
					},
				},
			},
		},
		{
			name: "ExternalCloudProviderDisabled",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ExternalCloudProviderConfig: &v1alpha1.ExternalCloudProviderConfig{},
				},
			},
		},
		{
			name: "ExternalCloudProviderExtraManifests",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ExternalCloudProviderConfig: &v1alpha1.ExternalCloudProviderConfig{
						ExternalManifests: []string{
							"https://www.example.com/manifest1.yaml",
							"https://www.example.com/manifest2.yaml",
						},
					},
				},
			},
			expectedError: "1 error occurred:\n\t* external cloud provider is disabled, but manifests are provided\n\n",
		},
		{
			name: "ExternalCloudProviderInvalidManifests",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ExternalCloudProviderConfig: &v1alpha1.ExternalCloudProviderConfig{
						ExternalEnabled: true,
						ExternalManifests: []string{
							"/manifest.yaml",
						},
					},
				},
			},
			expectedError: "1 error occurred:\n\t* invalid external cloud provider manifest url \"/manifest.yaml\": hostname must not be blank\n\n",
		},
		{
			name: "InlineManifests",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ClusterInlineManifests: v1alpha1.ClusterInlineManifests{
						{
							InlineManifestName: "",
						},
						{
							InlineManifestName: "foo",
						},
						{
							InlineManifestName: "bar",
						},
						{
							InlineManifestName: "foo",
						},
					},
				},
			},
			expectedError: "2 errors occurred:\n\t* inline manifest name can't be empty\n\t* inline manifest name \"foo\" is duplicate\n\n",
		},
		{
			name: "DeviceCIDRInvalid",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "eth0",
								DeviceCIDR:      "10.3.x",
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError:    "1 error occurred:\n\t* [networking.os.device.CIDR] \"eth0\": failed to parse IP address \"10.3.x\"\n\n",
			expectedWarnings: []string{"\"eth0\": machine.network.interface.cidr is deprecated, please use machine.network.interface.addresses"},
		},
		{
			name: "DeviceAddressInvalid",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "eth0",
								DeviceAddresses: []string{
									"192.168.0.5/24",
									"10.35.7.8",
									"10.3.x/24",
								},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError: "1 error occurred:\n\t* [networking.os.device.addresses] \"eth0\": invalid CIDR address: 10.3.x/24\n\n",
		},
		{
			name: "DeviceAddressAndCIDR",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "eth0",
								DeviceCIDR:      "192.24.3.45",
								DeviceAddresses: []string{
									"192.168.0.5/24",
								},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError:    "1 error occurred:\n\t* [networking.os.device] \"eth0\": interface can't have both .cidr and .addresses set\n\n",
			expectedWarnings: []string{"\"eth0\": machine.network.interface.cidr is deprecated, please use machine.network.interface.addresses"},
		},
		{
			name: "VlanCIDRInvalid",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "eth0",
								DeviceVlans: []*v1alpha1.Vlan{
									{
										VlanID:   25,
										VlanCIDR: "10.3.x",
									},
								},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError: "1 error occurred:\n\t* [networking.os.device.vlan.CIDR] eth0.25: failed to parse IP address \"10.3.x\"\n\n",
		},
		{
			name: "VlanAddressInvalid",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "eth0",
								DeviceVlans: []*v1alpha1.Vlan{
									{
										VlanID: 25,
										VlanAddresses: []string{
											"192.168.0.5/24",
											"10.35.7.8",
											"10.3.x/24",
										},
									},
								},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError: "1 error occurred:\n\t* [networking.os.device.vlan.addresses] eth0.25: invalid CIDR address: 10.3.x/24\n\n",
		},
		{
			name: "VlanAddressAndCIDR",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "eth0",
								DeviceVlans: []*v1alpha1.Vlan{
									{
										VlanID:   26,
										VlanCIDR: "192.24.3.45",
										VlanAddresses: []string{
											"192.168.0.5/24",
										},
									},
								},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError: "1 error occurred:\n\t* [networking.os.device.vlan] eth0.26: vlan can't have both .cidr and .addresses set\n\n",
		},
		{
			name: "BondDefaultConfig",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "bond0",
								DeviceBond:      &v1alpha1.Bond{},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
		},
		{
			name: "BondWrongMode",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "bond0",
								DeviceBond: &v1alpha1.Bond{
									BondMode:            "roundrobin",
									BondUpDelay:         100,
									BondPacketsPerSlave: 8,
									BondADActorSysPrio:  48,
									BondInterfaces: []string{
										"eth0",
										"eth1",
									},
								},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError: "3 errors occurred:\n\t* invalid bond type roundrobin\n\t* bond.upDelay can't be set if miiMon is zero\n\t* bond.adActorSysPrio is only available in 802.3ad mode\n\n",
		},
		{
			name: "BondDoubleBond",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "bond0",
								DeviceBond: &v1alpha1.Bond{
									BondInterfaces: []string{
										"eth0",
										"eth1",
									},
								},
							},
							{
								DeviceInterface: "bond1",
								DeviceBond: &v1alpha1.Bond{
									BondInterfaces: []string{
										"eth1",
										"eth2",
									},
								},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError: "1 error occurred:\n\t* interface \"eth1\" is declared as part of two bonds: \"bond0\" and \"bond1\"\n\n",
		},
		{
			name: "BondSlaveAddressing",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "bond0",
								DeviceBond: &v1alpha1.Bond{
									BondInterfaces: []string{
										"eth0",
										"eth1",
									},
								},
							},
							{
								DeviceInterface: "bond0",
								DeviceBond: &v1alpha1.Bond{
									BondInterfaces: []string{
										"eth0",
										"eth1",
									},
								},
							},
							{
								DeviceInterface: "eth0",
								DeviceDHCP:      true,
							},
							{
								DeviceInterface: "eth1",
								DeviceAddresses: []string{
									"192.168.0.1/24",
								},
							},
							{
								DeviceInterface: "eth2",
								DeviceCIDR:      "192.168.1.1/24",
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError: "2 errors occurred:\n\t* [networking.os.device] \"eth0\": bonded interface shouldn't have any addressing methods configured\n" +
				"\t* [networking.os.device] \"eth1\": bonded interface shouldn't have any addressing methods configured\n\n",
			expectedWarnings: []string{
				"\"eth2\": machine.network.interface.cidr is deprecated, please use machine.network.interface.addresses",
			},
		},
		{
			name: "Wireguard",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "wireguard0",
								DeviceWireguardConfig: &v1alpha1.DeviceWireguardConfig{
									WireguardPrivateKey: "ONtS+jU1Q+ZHLgs7DbYvnF5Iyj+koxBtvknDigFsdG8=",
									WireguardPeers: []*v1alpha1.DeviceWireguardPeer{
										{},
										{
											WireguardPublicKey: "4A3rogGVHuVjeZz5cbqryWXGkGBdIGC0E6+5mX2Iz1A=",
											WireguardEndpoint:  "example.com:1234",
											WireguardAllowedIPs: []string{
												"10.2.0.5/31",
												"2.4.5.3/32",
											},
										},
										{
											WireguardPublicKey: "4A3rogGVHuVjeZz5cbqryWXGkGBdIGC0E6+5mX2Iz1==",
											WireguardEndpoint:  "12.3.4.5",
											WireguardAllowedIPs: []string{
												"10.2.0",
											},
										},
									},
								},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError: "4 errors occurred:\n\t* public key invalid: wrong key \"\" length: 0\n\t* public key invalid: wrong key \"4A3rogGVHuVjeZz5cbqryWXGkGBdIGC0E6+5mX2Iz1==\" length: 31\n" +
				"\t* peer endpoint \"12.3.4.5\" is invalid\n\t* peer allowed IP \"10.2.0\" is invalid: invalid CIDR address: 10.2.0\n\n",
		},
		{
			name: "StaticRoutes",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "eth0",
								DeviceRoutes: []*v1alpha1.Route{
									{
										RouteGateway: "172.0.0.1",
									},
									{
										RouteNetwork: "10.0.0.0/24",
										RouteGateway: "10.0.0.1",
									},
									{
										RouteNetwork: "10.0.0.0/24",
										RouteGateway: "10.0.0.1",
										RouteSource:  "10.0.0.5",
									},
									{
										RouteGateway: "172.0.0.x",
									},
									{
										RouteNetwork: "10.0.0.0",
										RouteGateway: "10.0.0.1",
									},
									{
										RouteNetwork: "10.0.0.0/24",
										RouteGateway: "10.0.0.1",
										RouteSource:  "10.0.0.3/32",
									},
								},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError: "3 errors occurred:\n\t* [networking.os.device.route[3].gateway] \"172.0.0.x\": invalid network address\n" +
				"\t* [networking.os.device.route[4].network] \"10.0.0.0\": invalid network address\n" +
				"\t* [networking.os.device.route[5].source] \"10.0.0.3/32\": invalid network address\n\n",
		},
		{
			name: "KubeSpanNoDiscovery",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkKubeSpan: v1alpha1.NetworkKubeSpan{
							KubeSpanEnabled: true,
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError: "3 errors occurred:\n\t* .cluster.discovery should be enabled when .machine.network.kubespan is enabled\n" +
				"\t* .cluster.id should be set when .machine.network.kubespan is enabled\n" +
				"\t* .cluster.secret should be set when .machine.network.kubespan is enabled\n\n",
		},
		{
			name: "DiscoveryServiceEndpoint",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterID:     "foo",
					ClusterSecret: "bar",
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ClusterDiscoveryConfig: v1alpha1.ClusterDiscoveryConfig{
						DiscoveryEnabled: true,
						DiscoveryRegistries: v1alpha1.DiscoveryRegistriesConfig{
							RegistryService: v1alpha1.RegistryServiceConfig{
								RegistryEndpoint: "foo",
							},
						},
					},
				},
			},
			expectedError: "1 error occurred:\n\t* cluster discovery service registry endpoint is invalid: parse \"foo\": invalid URI for request\n\n",
		},
		{
			name: "DiscoveryServiceClusterIDSecret",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ClusterDiscoveryConfig: v1alpha1.ClusterDiscoveryConfig{
						DiscoveryEnabled: true,
					},
				},
			},
			expectedError: "2 errors occurred:\n\t* cluster discovery service requires .cluster.id\n\t* cluster discovery service requires .cluster.secret\n\n",
		},
		{
			name: "GoodEtcdSubnet",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					EtcdConfig: &v1alpha1.EtcdConfig{
						EtcdSubnet: "10.0.0.0/8",
					},
				},
			},
			expectedError: "",
		},
		{
			name: "BadEtcdSubnet",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					EtcdConfig: &v1alpha1.EtcdConfig{
						EtcdSubnet: "10.0.0.0",
					},
				},
			},
			expectedError: "1 error occurred:\n\t* \"10.0.0.0\" is not a valid subnet\n\n",
		},
		{
			name: "GoodKubeletSubnet",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineKubelet: &v1alpha1.KubeletConfig{
						KubeletNodeIP: v1alpha1.KubeletNodeIPConfig{
							KubeletNodeIPValidSubnets: []string{
								"10.0.0.0/8",
								"!10.0.0.3/32",
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError: "",
		},
		{
			name: "BadKubeletSubnet",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineKubelet: &v1alpha1.KubeletConfig{
						KubeletNodeIP: v1alpha1.KubeletNodeIPConfig{
							KubeletNodeIPValidSubnets: []string{
								"10.0.0.0",
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			expectedError: "1 error occurred:\n\t* kubelet nodeIP subnet is not valid: \"10.0.0.0\"\n\n",
		},
	} {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			opts := []config.ValidationOption{config.WithLocal()}
			if test.strict {
				opts = append(opts, config.WithStrict())
			}

			warnings, errrors := test.config.Validate(runtimeMode{test.requiresInstall}, opts...)

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError == "" {
				assert.NoError(t, errrors)
			} else {
				assert.EqualError(t, errrors, test.expectedError)
			}
		})
	}
}

func TestValidateCNI(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name             string
		config           *v1alpha1.CNIConfig
		expectedWarnings []string
		expectedError    string
	}{
		{
			name:          "Nil",
			expectedError: "", // Flannel is used by default
		},
		{
			name:          "Empty",
			config:        &v1alpha1.CNIConfig{},
			expectedError: "1 error occurred:\n\t* cni name should be one of [\"flannel\", \"custom\", \"none\"]\n\n",
		},
		{
			name: "FlannelNoManifests",
			config: &v1alpha1.CNIConfig{
				CNIName: constants.FlannelCNI,
			},
		},
		{
			name: "FlannelManifests",
			config: &v1alpha1.CNIConfig{
				CNIName: constants.FlannelCNI,
				CNIUrls: []string{
					"https://host.test/quick-install.yaml",
				},
			},
			expectedError: "1 error occurred:\n\t* \"urls\" field should be empty for \"flannel\" CNI\n\n",
		},
		{
			name: "CustomNoManifests",
			config: &v1alpha1.CNIConfig{
				CNIName: constants.CustomCNI,
			},
			expectedWarnings: []string{
				"\"urls\" field should not be empty for \"custom\" CNI",
			},
		},
		{
			name: "CustomManifests",
			config: &v1alpha1.CNIConfig{
				CNIName: constants.CustomCNI,
				CNIUrls: []string{
					"https://host.test/quick-install.yaml",
				},
			},
		},
		{
			name: "NoneNoManifests",
			config: &v1alpha1.CNIConfig{
				CNIName: constants.NoneCNI,
			},
		},
		{
			name: "NoneManifests",
			config: &v1alpha1.CNIConfig{
				CNIName: constants.NoneCNI,
				CNIUrls: []string{
					"https://host.test/quick-install.yaml",
				},
			},
			expectedError: "1 error occurred:\n\t* \"urls\" field should be empty for \"none\" CNI\n\n",
		},
	} {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var cc v1alpha1.ClusterConfig
			if test.config != nil {
				cc.ClusterNetwork = &v1alpha1.ClusterNetworkConfig{
					CNI: test.config,
				}
			}

			cni := cc.CNI()
			require.NotNil(t, cni, "CNI() method should return default value for empty config")

			warnings, errrors := v1alpha1.ValidateCNI(cni)

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError == "" {
				assert.NoError(t, errrors)
			} else {
				assert.EqualError(t, errrors, test.expectedError)
			}
		})
	}
}
