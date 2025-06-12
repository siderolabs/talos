// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
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

func (runtimeMode) InContainer() bool {
	return false
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
				MachineConfig: &v1alpha1.MachineConfig{
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
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
			expectedWarnings: []string{
				`use "worker" instead of "join" for machine type`,
			},
		},
		{
			name: "NoMachineTypeStrict",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
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
			strict:        true,
			expectedError: "1 error occurred:\n\t* warning: use \"worker\" instead of \"\" for machine type\n\n",
		},
		{
			name: "WorkerNoAcceptedCAs",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineCA:   &x509.PEMEncodedCertificateAndKey{},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
				},
			},
			strict:        true,
			expectedError: "1 error occurred:\n\t* trusted CA certificates are required on non-controlplane nodes (.machine.ca.crt, .machine.acceptedCAs)\n\n",
		},
		{
			name: "WorkerOnlyAcceptedCAs",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineAcceptedCAs: []*x509.PEMEncodedCertificate{
						{
							Crt: []byte("foo"),
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
			strict: true,
		},
		{
			name: "ControlplaneNoCAKey",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
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
			strict:        true,
			expectedError: "1 error occurred:\n\t* issuing CA key is required for controlplane nodes (.machine.ca.key)\n\n",
		},
		{
			name: "NoMachineInstall",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
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
			name: "NoMachineInstallRequired",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
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
			expectedError:   "1 error occurred:\n\t* install instructions are required in \"runtimeMode(true)\" mode\n\n",
		},
		{
			name: "MachineInstallDisk",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
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
			name: "MachineInstallExtensionsDuplicate",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
					MachineInstall: &v1alpha1.InstallConfig{
						InstallDisk: "/dev/vda",
						InstallExtensions: []v1alpha1.InstallExtensionConfig{
							{
								ExtensionImage: "ghcr.io/siderolabs/gvisor:v0.1.0",
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
			requiresInstall: true,
			expectedWarnings: []string{
				".machine.install.extensions is deprecated, please see https://www.talos.dev/latest/talos-guides/install/boot-assets/",
			},
		},
		{
			name: "ExternalCloudProviderEnabled",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ExternalCloudProviderConfig: &v1alpha1.ExternalCloudProviderConfig{
						ExternalEnabled: pointer.To(true),
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ExternalCloudProviderConfig: &v1alpha1.ExternalCloudProviderConfig{
						ExternalEnabled: pointer.To(true),
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ExternalCloudProviderConfig: &v1alpha1.ExternalCloudProviderConfig{
						ExternalEnabled: pointer.To(true),
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
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
			name: "DNSDomainEmpty",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ClusterNetwork: &v1alpha1.ClusterNetworkConfig{},
				},
			},
		},
		{
			name: "DeviceCIDRInvalid",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
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
			name: "BondInterfacesAndSelectors",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "bond0",
								DeviceBond: &v1alpha1.Bond{
									BondInterfaces: []string{
										"eth0",
										"eth1",
									},
									BondDeviceSelectors: []v1alpha1.NetworkDeviceSelector{
										{
											NetworkDeviceKernelDriver: "virtio",
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
			expectedError: "1 error occurred:\n\t* interface \"bond0\" has both interfaces and selectors set: config sections are mutually exclusive\n\n",
		},
		{
			name: "BondDoubleBond",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
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
			expectedError: "1 error occurred:\n\t* interface \"eth1\" is declared as part of two separate links: \"bond0\" and \"bond1\"\n\n",
		},
		{
			name: "BondSlaveAddressing",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "bond0",
								DeviceBond: &v1alpha1.Bond{
									BondInterfaces: []string{
										"eth0",
										"eth1",
										"eth2",
									},
								},
							},
							{
								DeviceInterface: "eth0",
								DeviceDHCP:      pointer.To(true),
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
			expectedError: "3 errors occurred:\n" +
				"\t* [networking.os.device] \"eth0\": bonded/bridged interface shouldn't have any addressing methods configured\n" +
				"\t* [networking.os.device] \"eth1\": bonded/bridged interface shouldn't have any addressing methods configured\n" +
				"\t* [networking.os.device] \"eth2\": bonded/bridged interface shouldn't have any addressing methods configured\n" +
				"\n",
			expectedWarnings: []string{
				"\"eth2\": machine.network.interface.cidr is deprecated, please use machine.network.interface.addresses",
			},
		},
		{
			name: "BridgeSlaveAddressing",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "br0",
								DeviceBridge: &v1alpha1.Bridge{
									BridgedInterfaces: []string{
										"eth0",
										"eth1",
										"eth2",
									},
								},
							},
							{
								DeviceInterface: "eth0",
								DeviceDHCP:      pointer.To(true),
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
			expectedError: "3 errors occurred:\n" +
				"\t* [networking.os.device] \"eth0\": bonded/bridged interface shouldn't have any addressing methods configured\n" +
				"\t* [networking.os.device] \"eth1\": bonded/bridged interface shouldn't have any addressing methods configured\n" +
				"\t* [networking.os.device] \"eth2\": bonded/bridged interface shouldn't have any addressing methods configured\n" +
				"\n",
			expectedWarnings: []string{
				"\"eth2\": machine.network.interface.cidr is deprecated, please use machine.network.interface.addresses",
			},
		},
		{
			name: "BridgeDoubleBridge",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "br0",
								DeviceBridge: &v1alpha1.Bridge{
									BridgedInterfaces: []string{
										"eth0",
										"eth1",
									},
								},
							},
							{
								DeviceInterface: "br1",
								DeviceBridge: &v1alpha1.Bridge{
									BridgedInterfaces: []string{
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
			expectedError: "1 error occurred:\n\t* interface \"eth1\" is declared as part of two separate links: \"br0\" and \"br1\"\n\n",
		},
		{
			name: "InterfaceIsBothBridgeAndBond",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "bridgebond0",
								DeviceBridge: &v1alpha1.Bridge{
									BridgedInterfaces: []string{
										"eth0",
										"eth1",
									},
								},
								DeviceBond: &v1alpha1.Bond{
									BondInterfaces: []string{
										"eth2",
										"eth3",
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
			expectedError: "1 error occurred:\n\t* interface has both bridge and bond sections set \"bridgebond0\": config sections are mutually exclusive\n\n",
		},
		{
			name: "InterfacePartOfBridgeAndBond",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "br0",
								DeviceBridge: &v1alpha1.Bridge{
									BridgedInterfaces: []string{
										"eth0",
										"eth1",
									},
								},
							},
							{
								DeviceInterface: "bond0",
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
			expectedError: "1 error occurred:\n\t* interface \"eth1\" is declared as part of two separate links: \"br0\" and \"bond0\"\n\n",
		},
		{
			name: "Wireguard",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
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
									{
										RouteNetwork: "169.254.254.254/32",
									},
									{
										RouteSource: "10.0.0.3",
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
			expectedError: "4 errors occurred:\n\t* [networking.os.device.route[3].gateway] \"172.0.0.x\": invalid network address\n" +
				"\t* [networking.os.device.route[4].network] \"10.0.0.0\": invalid network address\n" +
				"\t* [networking.os.device.route[5].source] \"10.0.0.3/32\": invalid network address\n" +
				"\t* [networking.os.device.route[7]]: either network or gateway should be set\n\n",
		},
		{
			name: "KubeSpanNoDiscovery",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkKubeSpan: &v1alpha1.NetworkKubeSpan{
							KubeSpanEnabled: pointer.To(true),
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterID:     "foo",
					ClusterSecret: "bar",
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{
						DiscoveryEnabled: pointer.To(true),
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{
						DiscoveryEnabled: pointer.To(true),
					},
				},
			},
			expectedError: "2 errors occurred:\n\t* cluster discovery service requires .cluster.id\n\t* cluster discovery service requires .cluster.secret\n\n",
		},
		{
			name: "EtcdMissingCa",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					EtcdConfig: &v1alpha1.EtcdConfig{},
				},
			},
			expectedError: "1 error occurred:\n\t* key/cert combination should not be empty\n\n",
		},
		{
			name: "EtcdConfigProvidedForWorker",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					EtcdConfig: &v1alpha1.EtcdConfig{},
				},
			},
			expectedError: "1 error occurred:\n\t* etcd config is only allowed on control plane machines\n\n",
		},
		{
			name: "GoodEtcdSubnet",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					EtcdConfig: &v1alpha1.EtcdConfig{
						RootCA: &x509.PEMEncodedCertificateAndKey{},
						EtcdAdvertisedSubnets: []string{
							"10.0.0.0/8",
							"!1.1.1.1/32",
						},
						EtcdListenSubnets: []string{
							"10.0.0.0/8",
							"1.1.1.1/32",
						},
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					EtcdConfig: &v1alpha1.EtcdConfig{
						RootCA: &x509.PEMEncodedCertificateAndKey{},
						EtcdAdvertisedSubnets: []string{
							"1234:",
						},
						EtcdListenSubnets: []string{
							"10",
						},
					},
				},
			},
			expectedError: "2 errors occurred:\n\t* etcd advertised subnet is not valid: \"1234:\"\n\t* etcd listen subnet is not valid: \"10\"\n\n",
		},
		{
			name: "GoodKubeletSubnet",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
					MachineKubelet: &v1alpha1.KubeletConfig{
						KubeletNodeIP: &v1alpha1.KubeletNodeIPConfig{
							KubeletNodeIPValidSubnets: []string{
								"10.0.0.0/8",
								"!10.0.0.3/32",
								"!fd00::169:254:2:53/128",
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
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
					MachineKubelet: &v1alpha1.KubeletConfig{
						KubeletNodeIP: &v1alpha1.KubeletNodeIPConfig{
							KubeletNodeIPValidSubnets: []string{
								"10.0.0.0.3",
								"[fd00::169:254:2:53]:344",
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
			expectedError: "2 errors occurred:\n" +
				"\t* kubelet nodeIP subnet is not valid: \"10.0.0.0.3\"\n" +
				"\t* kubelet nodeIP subnet is not valid: \"[fd00::169:254:2:53]:344\"\n" +
				"\n",
		},
		{
			name: "BadKubeletExtraConfig",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineAcceptedCAs: []*x509.PEMEncodedCertificate{
						{
							Crt: []byte("foo"),
						},
					},
					MachineKubelet: &v1alpha1.KubeletConfig{
						KubeletExtraConfig: v1alpha1.Unstructured{
							Object: map[string]any{
								"port": 345,
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
			expectedError: "1 error occurred:\n\t* kubelet configuration field \"port\" can't be overridden\n\n",
		},
		{
			name: "DeviceInterfaceInvalid",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{},
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
			expectedError: "1 error occurred:\n\t* [networking.os.device.interface], [networking.os.device.deviceSelector]: required either config section to be set\n\n",
		},
		{
			name: "DeviceSelectorAndInterfaceSet",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "eth0",
								DeviceSelector: &v1alpha1.NetworkDeviceSelector{
									NetworkDeviceBus: "00:01",
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
			expectedError: "1 error occurred:\n\t* [networking.os.device.interface], [networking.os.device.deviceSelector]: config sections are mutually exclusive\n\n",
		},
		{
			name: "DeviceSelectorAndInterfaceSet",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceSelector: &v1alpha1.NetworkDeviceSelector{},
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
			expectedError: "1 error occurred:\n\t* [networking.os.device.deviceSelector]: config section should contain at least one field\n\n",
		},
		{
			name: "TalosAPIAccessRBAC",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineFeatures: &v1alpha1.FeaturesConfig{
						KubernetesTalosAPIAccessConfig: &v1alpha1.KubernetesTalosAPIAccessConfig{
							AccessEnabled: pointer.To(true),
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
			expectedError: "1 error occurred:\n\t* feature API RBAC should be enabled when Kubernetes Talos API Access feature is enabled\n\n",
		},
		{
			name: "TalosAPIAccessWorker",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
					MachineFeatures: &v1alpha1.FeaturesConfig{
						RBAC: pointer.To(true),
						KubernetesTalosAPIAccessConfig: &v1alpha1.KubernetesTalosAPIAccessConfig{
							AccessEnabled: pointer.To(true),
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
			expectedError: "1 error occurred:\n\t* feature Kubernetes Talos API Access can only be enabled on control plane machines\n\n",
		},
		{
			name: "TalosAPIAccessInvalidRole",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineFeatures: &v1alpha1.FeaturesConfig{
						RBAC: pointer.To(true),
						KubernetesTalosAPIAccessConfig: &v1alpha1.KubernetesTalosAPIAccessConfig{
							AccessEnabled: pointer.To(true),
							AccessAllowedRoles: []string{
								"os:reader",
								"invalid:role1",
								"os:etcd:backup",
								"invalid:role2",
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
			expectedError: "2 errors occurred:\n\t* invalid role \"invalid:role1\" in allowed roles for " +
				"Kubernetes Talos API Access\n\t* invalid role \"invalid:role2\" in allowed roles for " +
				"Kubernetes Talos API Access\n\n",
		},
		{
			name: "NodeLabels",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
					MachineNodeLabels: map[string]string{
						"/foo":          "bar",
						"key":           "value",
						"talos.dev/foo": "bar",
						"@!":            "#$",
						"123@.dev/":     "456",
						"a/b/c":         strings.Repeat("a", 64),
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
			expectedError: "1 error occurred:\n\t* invalid machine node labels: 6 errors occurred:\n\t* prefix cannot be empty: \"/foo\"\n\t* prefix \"123@.dev\" is invalid: domain doesn't match required format: \"123@.dev\"\n\t* name \"@!\" is invalid\n\t* label value \"#$\" is invalid\n\t* invalid format: too many slashes: \"a/b/c\"\n\t* label value length exceeds limit of 63: \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"\n\n\n\n", //nolint:lll
		},
		{
			name: "GoodKubeSpanEndpointFilters",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkKubeSpan: &v1alpha1.NetworkKubeSpan{
							KubeSpanEnabled: pointer.To(true),
							KubeSpanFilters: &v1alpha1.KubeSpanFilters{
								KubeSpanFiltersEndpoints: []string{
									"0.0.0.0/0",
									"::/0",
									"!192.168.0.0/16",
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
					ClusterID:     "test",
					ClusterSecret: "test",
					ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{
						DiscoveryEnabled: pointer.To(true),
					},
				},
			},
			expectedError: "",
		},
		{
			name: "BadKubeSpanEndpointFilters",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkKubeSpan: &v1alpha1.NetworkKubeSpan{
							KubeSpanEnabled: pointer.To(true),
							KubeSpanFilters: &v1alpha1.KubeSpanFilters{
								KubeSpanFiltersEndpoints: []string{
									"!10",
									"123::/456",
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
					ClusterID:     "test",
					ClusterSecret: "test",
					ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{
						DiscoveryEnabled: pointer.To(true),
					},
				},
			},
			expectedError: "2 errors occurred:\n\t* KubeSpan endpoint filer is not valid: \"10\"\n\t* KubeSpan endpoint filer is not valid: \"123::/456\"\n\n",
		},
		{
			name: "KubeSpanSmallMTU",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
					},
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkKubeSpan: &v1alpha1.NetworkKubeSpan{
							KubeSpanEnabled: pointer.To(true),
							KubeSpanMTU:     pointer.To(uint32(576)),
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					ClusterID:     "test",
					ClusterSecret: "test",
					ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{
						DiscoveryEnabled: pointer.To(true),
					},
				},
			},
			expectedError: "1 error occurred:\n\t* kubespan link MTU must be at least 1280\n\n",
		},
		{
			name: "ControlPlanePodResources",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					APIServerConfig: &v1alpha1.APIServerConfig{
						ResourcesConfig: &v1alpha1.ResourcesConfig{
							Requests: v1alpha1.Unstructured{
								Object: map[string]any{
									"cpu":      "1m",
									"invalid1": "23",
								},
							},
						},
					},
					ControllerManagerConfig: &v1alpha1.ControllerManagerConfig{
						ResourcesConfig: &v1alpha1.ResourcesConfig{
							Limits: v1alpha1.Unstructured{
								Object: map[string]any{
									"memory":   "1m",
									"invalid2": "23",
								},
							},
						},
					},
					SchedulerConfig: &v1alpha1.SchedulerConfig{
						ResourcesConfig: &v1alpha1.ResourcesConfig{
							Requests: v1alpha1.Unstructured{
								Object: map[string]any{
									"cpu": "1m",
								},
							},
							Limits: v1alpha1.Unstructured{
								Object: map[string]any{
									"invalid3": "23",
								},
							},
						},
					},
				},
			},
			expectedError: "3 errors occurred:\n\t* apiserver resource validation failed: unsupported pod resource \"invalid1\"\n\t* controller-manager resource validation failed: unsupported pod resource \"invalid2\"\n\t* scheduler resource validation failed: unsupported pod resource \"invalid3\"\n\n", //nolint:lll
		},
		{
			name: "ControlPlaneInvalidAuthorizationConfig",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					APIServerConfig: &v1alpha1.APIServerConfig{
						AuthorizationConfigConfig: []*v1alpha1.AuthorizationConfigAuthorizerConfig{
							{
								AuthorizerName: "foo",
							},
						},
					},
				},
			},
			expectedError: "1 error occurred:\n\t* apiserver authorization config validation failed: authorizer type must be set\n\n",
		},
		{
			name: "ControlPlaneAuthorizationConfigWithAuthorizationModeFlag",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					APIServerConfig: &v1alpha1.APIServerConfig{
						AuthorizationConfigConfig: []*v1alpha1.AuthorizationConfigAuthorizerConfig{},
						ExtraArgsConfig: map[string]string{
							"authorization-mode": "Node",
						},
					},
				},
			},
			expectedError: "1 error occurred:\n\t* authorization-mode cannot be used in conjunction with AuthorizationConfig, use eitherr AuthorizationConfig or authorization-mode\n\n",
		},
		{
			name: "ControlPlaneAuthorizationConfigWithAuthorizationWebhook",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							endpointURL,
						},
					},
					APIServerConfig: &v1alpha1.APIServerConfig{
						AuthorizationConfigConfig: []*v1alpha1.AuthorizationConfigAuthorizerConfig{},
						ExtraArgsConfig: map[string]string{
							"authorization-webhook-version": "v1",
						},
					},
				},
			},
			expectedError: "1 error occurred:\n\t* authorization-webhook-* flags cannot be used in conjunction with AuthorizationConfig, use either AuthorizationConfig or authorization-webhook-* flags\n\n",
		},
		{
			name: "MachineBaseRuntimeSpecOverrides",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineBaseRuntimeSpecOverrides: v1alpha1.Unstructured{
						Object: map[string]any{
							"process": map[string]any{
								"rlimits": []map[string]any{
									{
										"type": "RLIMIT_NOFILE",
										"hard": 1024,
										"soft": 1024,
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
		},
		{
			name: "MachineFeaturesInvalidAddressSortingAlgorithm",
			config: &v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
					MachineFeatures: &v1alpha1.FeaturesConfig{
						FeatureNodeAddressSortAlgorithm: "xyz",
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
			expectedError: "1 error occurred:\n\t* invalid node address sort algorithm: xyz does not belong to AddressSortAlgorithm values\n\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			opts := []validation.Option{validation.WithLocal()}
			if test.strict {
				opts = append(opts, validation.WithStrict())
			}

			warnings, errors := test.config.Validate(runtimeMode{test.requiresInstall}, opts...)

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError == "" {
				assert.NoError(t, errors)
			} else {
				assert.EqualError(t, errors, test.expectedError)
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
			name: "FlannelExtraArgs",
			config: &v1alpha1.CNIConfig{
				CNIName: constants.FlannelCNI,
				CNIFlannel: &v1alpha1.FlannelCNIConfig{
					FlanneldExtraArgs: []string{"--foo"},
				},
			},
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
			name: "CustomFlannelExtraArgs",
			config: &v1alpha1.CNIConfig{
				CNIName: constants.CustomCNI,
				CNIUrls: []string{
					"https://host.test/quick-install.yaml",
				},
				CNIFlannel: &v1alpha1.FlannelCNIConfig{
					FlanneldExtraArgs: []string{"--foo"},
				},
			},
			expectedError: "1 error occurred:\n\t* \"flanneldExtraArgs\" field should be empty for \"custom\" CNI\n\n",
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

func TestKubernetesVersionFromImageRef(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		imageRef string

		expectedVersion string
	}{
		{
			imageRef:        "ghcr.io/siderolabs/kubelet:v1.32.2",
			expectedVersion: "1.32.2",
		},
		{
			imageRef:        "ghcr.io/siderolabs/kubelet:v1.32.2@sha256:123456",
			expectedVersion: "1.32.2",
		},
	} {
		t.Run(test.imageRef, func(t *testing.T) {
			t.Parallel()

			version, err := v1alpha1.KubernetesVersionFromImageRef(test.imageRef)
			require.NoError(t, err)

			assert.Equal(t, test.expectedVersion, version.String())
		})
	}
}

func TestRuntimeValidate(t *testing.T) {
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
			name: "valid",
			config: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: endpointURL,
						},
					},
				},
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
				},
			},
		},
		{
			name: "old kubelet version",
			config: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: endpointURL,
						},
					},
				},
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "worker",
					MachineKubelet: &v1alpha1.KubeletConfig{
						KubeletImage: constants.KubeletImage + ":v1.24.0",
					},
				},
			},
			expectedError: "1 error occurred:\n\t* kubelet image is not valid: version of Kubernetes 1.24.0 is too old to be used with Talos VERSION\n\n",
		},
		{
			name: "old api-server version",
			config: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: endpointURL,
						},
					},
					APIServerConfig: &v1alpha1.APIServerConfig{
						ContainerImage: constants.KubernetesAPIServerImage + ":v1.24.0",
					},
				},
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineKubelet: &v1alpha1.KubeletConfig{
						KubeletImage: constants.KubeletImage + ":v" + constants.DefaultKubernetesVersion,
					},
				},
			},
			expectedError: "1 error occurred:\n\t* kube-apiserver image is not valid: version of Kubernetes 1.24.0 is too old to be used with Talos VERSION\n\n",
		},
		{
			name: "old controller-manager and scheduler version",
			config: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: endpointURL,
						},
					},
					APIServerConfig: &v1alpha1.APIServerConfig{
						ContainerImage: constants.KubernetesAPIServerImage + ":v" + constants.DefaultKubernetesVersion,
					},
					ControllerManagerConfig: &v1alpha1.ControllerManagerConfig{
						ContainerImage: constants.KubernetesControllerManagerImage + ":v1.24.0",
					},
					SchedulerConfig: &v1alpha1.SchedulerConfig{
						ContainerImage: constants.KubernetesSchedulerImage + ":v1.24.0",
					},
				},
				MachineConfig: &v1alpha1.MachineConfig{
					MachineType: "controlplane",
					MachineKubelet: &v1alpha1.KubeletConfig{
						KubeletImage: constants.KubeletImage + ":v" + constants.DefaultKubernetesVersion,
					},
				},
			},
			expectedError: "2 errors occurred:\n\t* kube-controller-manager image is not valid: version of Kubernetes 1.24.0 is too old to be used with Talos VERSION\n\t* kube-scheduler image is not valid: version of Kubernetes 1.24.0 is too old to be used with Talos VERSION\n\n", //nolint:lll
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var opts []validation.Option
			if test.strict {
				opts = append(opts, validation.WithStrict())
			}

			st := state.WrapCore(inmem.NewState(""))

			warnings, errors := test.config.RuntimeValidate(t.Context(), st, runtimeMode{test.requiresInstall}, opts...)

			assert.Equal(t, test.expectedWarnings, warnings)

			currentTalosVersion, err := compatibility.ParseTalosVersion(version.NewVersion())
			require.NoError(t, err)

			if test.expectedError == "" {
				assert.NoError(t, errors)
			} else {
				test.expectedError = strings.ReplaceAll(test.expectedError, "VERSION", currentTalosVersion.String())

				assert.EqualError(t, errors, test.expectedError)
			}
		})
	}
}
