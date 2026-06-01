// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//nolint:gocyclo
func TestRedactSecrets(t *testing.T) {
	t.Parallel()

	for _, versionContract := range []*config.VersionContract{
		config.TalosVersion1_13,
		config.TalosVersion1_14,
	} {
		t.Run(versionContract.String(), func(t *testing.T) {
			t.Parallel()

			input, err := generate.NewInput("test", "https://doesntmatter:6443", constants.DefaultKubernetesVersion, generate.WithVersionContract(versionContract))
			require.NoError(t, err)

			container, err := input.Config(machine.TypeControlPlane)
			if err != nil {
				return
			}

			config := container.RawV1Alpha1()

			require.NotEmpty(t, config.MachineConfig.MachineToken)
			require.NotEmpty(t, config.MachineConfig.MachineCA.Key)
			require.NotEmpty(t, config.ClusterConfig.ClusterSecret)
			require.NotEmpty(t, config.ClusterConfig.BootstrapToken)
			require.Empty(t, config.ClusterConfig.ClusterAESCBCEncryptionSecret)

			require.NotEmpty(t, config.ClusterConfig.ClusterCA.Key)
			require.NotEmpty(t, config.ClusterConfig.EtcdConfig.RootCA.Key)
			require.NotEmpty(t, config.ClusterConfig.ClusterServiceAccount.Key)

			if !versionContract.MultidocKubernetesConfigSupported() {
				require.NotEmpty(t, config.ClusterConfig.ClusterSecretboxEncryptionSecret)
			}

			replacement := "**.***"

			config.Redact(replacement)

			require.Equal(t, replacement, config.Machine().Security().Token())
			require.Equal(t, replacement, string(config.Machine().Security().IssuingCA().Key))
			require.Equal(t, replacement, config.Cluster().Secret())
			require.Equal(t, "***", config.Cluster().Token().Secret())
			require.Equal(t, "", config.Cluster().AESCBCEncryptionSecret())
			require.Equal(t, replacement, string(config.Cluster().IssuingCA().Key))
			require.Equal(t, replacement, string(config.Cluster().Etcd().CA().Key))
			require.Equal(t, replacement, string(config.Cluster().ServiceAccount().Key))

			if versionContract.MultidocKubernetesConfigSupported() {
				require.Empty(t, config.Cluster().SecretboxEncryptionSecret())
			} else {
				require.Equal(t, replacement, config.Cluster().SecretboxEncryptionSecret())
			}
		})
	}
}

//nolint:gocyclo
func TestRedactExtendedSecrets(t *testing.T) {
	t.Parallel()

	cfg := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineRegistries: v1alpha1.RegistriesConfig{
				RegistryConfig: map[string]*v1alpha1.RegistryConfig{
					"my-registry.local:5000": {
						RegistryAuth: &v1alpha1.RegistryAuthConfig{
							RegistryUsername:      "alice",
							RegistryPassword:      "topsecret",
							RegistryAuth:          "raw-auth",
							RegistryIdentityToken: "id-token",
						},
						RegistryTLS: &v1alpha1.RegistryTLSConfig{
							TLSClientIdentity: &x509.PEMEncodedCertificateAndKey{
								Crt: []byte("cert"),
								Key: []byte("private-key"),
							},
							TLSCA: []byte("ca-bundle"),
						},
					},
					"empty-registry.local": nil,
				},
			},
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: v1alpha1.NetworkDeviceList{
					{
						DeviceInterface: "eth0",
						DeviceWireguardConfig: &v1alpha1.DeviceWireguardConfig{
							WireguardPrivateKey: "wg-priv-key",
						},
						DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
							EquinixMetalConfig: &v1alpha1.VIPEquinixMetalConfig{
								EquinixMetalAPIToken: "equinix-token",
							},
							HCloudConfig: &v1alpha1.VIPHCloudConfig{
								HCloudAPIToken: "hcloud-token",
							},
						},
						DeviceVlans: v1alpha1.VlanList{
							{
								VlanID: 42,
								VlanVIP: &v1alpha1.DeviceVIPConfig{
									EquinixMetalConfig: &v1alpha1.VIPEquinixMetalConfig{
										EquinixMetalAPIToken: "equinix-vlan-token",
									},
									HCloudConfig: &v1alpha1.VIPHCloudConfig{
										HCloudAPIToken: "hcloud-vlan-token",
									},
								},
							},
							nil,
						},
					},
					nil,
				},
			},
			MachineSystemDiskEncryption: &v1alpha1.SystemDiskEncryptionConfig{
				StatePartition: &v1alpha1.EncryptionConfig{
					EncryptionKeys: []*v1alpha1.EncryptionKey{
						{KeyStatic: &v1alpha1.EncryptionKeyStatic{KeyData: "state-passphrase"}},
						{KeyNodeID: &v1alpha1.EncryptionKeyNodeID{}},
						nil,
					},
				},
				EphemeralPartition: &v1alpha1.EncryptionConfig{
					EncryptionKeys: []*v1alpha1.EncryptionKey{
						{KeyStatic: &v1alpha1.EncryptionKeyStatic{KeyData: "ephemeral-passphrase"}},
					},
				},
			},
		},
	}

	const replacement = "**.***"

	cfg.Redact(replacement)

	registry := cfg.MachineConfig.MachineRegistries.RegistryConfig["my-registry.local:5000"]
	require.Equal(t, "alice", registry.RegistryAuth.RegistryUsername, "username is not a secret and must not be redacted")
	require.Equal(t, replacement, registry.RegistryAuth.RegistryPassword)
	require.Equal(t, replacement, registry.RegistryAuth.RegistryAuth)
	require.Equal(t, replacement, registry.RegistryAuth.RegistryIdentityToken)
	require.Equal(t, "cert", string(registry.RegistryTLS.TLSClientIdentity.Crt), "client cert is public and must not be redacted")
	require.Equal(t, replacement, string(registry.RegistryTLS.TLSClientIdentity.Key))
	require.Equal(t, "ca-bundle", string(registry.RegistryTLS.TLSCA), "CA bundle is public and must not be redacted")

	device := cfg.MachineConfig.MachineNetwork.NetworkInterfaces[0]
	require.Equal(t, replacement, device.DeviceWireguardConfig.WireguardPrivateKey)
	require.Equal(t, replacement, device.DeviceVIPConfig.EquinixMetalConfig.EquinixMetalAPIToken)
	require.Equal(t, replacement, device.DeviceVIPConfig.HCloudConfig.HCloudAPIToken)

	vlan := device.DeviceVlans[0]
	require.Equal(t, replacement, vlan.VlanVIP.EquinixMetalConfig.EquinixMetalAPIToken)
	require.Equal(t, replacement, vlan.VlanVIP.HCloudConfig.HCloudAPIToken)

	require.Equal(t, replacement, cfg.MachineConfig.MachineSystemDiskEncryption.StatePartition.EncryptionKeys[0].KeyStatic.KeyData)
	require.Equal(t, replacement, cfg.MachineConfig.MachineSystemDiskEncryption.EphemeralPartition.EncryptionKeys[0].KeyStatic.KeyData)
}
