// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"net/netip"
	"net/url"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	sideroLinkCfg := siderolink.NewConfigV1Alpha1()
	sideroLinkCfg.APIUrlConfig.URL = must.Value(url.Parse("https://siderolink.api/?jointoken=secret&user=alice"))(t)

	invalidSideroLinkCfg := siderolink.NewConfigV1Alpha1()

	v1alpha1Cfg := &v1alpha1.Config{
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{
				Endpoint: &v1alpha1.Endpoint{
					URL: must.Value(url.Parse("https://localhost:6443"))(t),
				},
			},
		},
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "worker",
			MachineCA: &x509.PEMEncodedCertificateAndKey{
				Crt: []byte("cert"),
			},
		},
	}

	invalidV1alpha1Config := &v1alpha1.Config{}

	for _, tt := range []struct {
		name      string
		documents []config.Document

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
		},
		{
			name:      "multi-doc",
			documents: []config.Document{sideroLinkCfg, v1alpha1Cfg},
		},
		{
			name:      "only siderolink",
			documents: []config.Document{sideroLinkCfg},
		},
		{
			name:      "only v1alpha1",
			documents: []config.Document{v1alpha1Cfg},
		},
		{
			name:          "invalid siderolink",
			documents:     []config.Document{invalidSideroLinkCfg},
			expectedError: "1 error occurred:\n\t* SideroLinkConfig: apiUrl is required\n\n",
		},
		{
			name:          "invalid v1alpha1",
			documents:     []config.Document{invalidV1alpha1Config},
			expectedError: "1 error occurred:\n\t* v1alpha1.Config: 1 error occurred:\n\t* machine instructions are required\n\n\n\n",
		},
		{
			name:          "invalid multi-doc",
			documents:     []config.Document{invalidSideroLinkCfg, invalidV1alpha1Config},
			expectedError: "2 errors occurred:\n\t* v1alpha1.Config: 1 error occurred:\n\t* machine instructions are required\n\n\n\t* SideroLinkConfig: apiUrl is required\n\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(tt.documents...)
			require.NoError(t, err)

			warnings, err := ctr.Validate(validationMode{})

			if tt.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.expectedError)
			}

			require.Equal(t, tt.expectedWarnings, warnings)
		})
	}
}

func TestCrossValidateEncryption(t *testing.T) {
	t.Parallel()

	v1alpha1Cfg := &v1alpha1.Config{
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{
				Endpoint: &v1alpha1.Endpoint{
					URL: must.Value(url.Parse("https://localhost:6443"))(t),
				},
			},
		},
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "worker",
			MachineCA: &x509.PEMEncodedCertificateAndKey{
				Crt: []byte("cert"),
			},
			MachineSystemDiskEncryption: &v1alpha1.SystemDiskEncryptionConfig{
				EphemeralPartition: &v1alpha1.EncryptionConfig{
					EncryptionKeys: []*v1alpha1.EncryptionKey{
						{
							KeySlot: 1,
							KeyStatic: &v1alpha1.EncryptionKeyStatic{
								KeyData: "static-key",
							},
						},
					},
				},
			},
		},
	}

	defaultEphemeral := block.NewVolumeConfigV1Alpha1()
	defaultEphemeral.MetaName = constants.EphemeralPartitionLabel

	encryptedEphemeral := block.NewVolumeConfigV1Alpha1()
	encryptedEphemeral.MetaName = constants.EphemeralPartitionLabel
	encryptedEphemeral.EncryptionSpec = block.EncryptionSpec{
		EncryptionProvider: blockres.EncryptionProviderLUKS2,
		EncryptionKeys: []block.EncryptionKey{
			{
				KeySlot: 2,
				KeyStatic: &block.EncryptionKeyStatic{
					KeyData: "encrypted-static-key",
				},
			},
		},
	}

	encryptedState := block.NewVolumeConfigV1Alpha1()
	encryptedState.MetaName = constants.StatePartitionLabel
	encryptedState.EncryptionSpec = block.EncryptionSpec{
		EncryptionProvider: blockres.EncryptionProviderLUKS2,
		EncryptionKeys: []block.EncryptionKey{
			{
				KeySlot: 3,
				KeyTPM:  &block.EncryptionKeyTPM{},
			},
		},
	}

	for _, tt := range []struct {
		name      string
		documents []config.Document

		expectedError    string
		expectedWarnings []string
	}{
		{
			name:      "only v1alpha1",
			documents: []config.Document{v1alpha1Cfg},
		},
		{
			name:      "v1alpha1 with no-conflict volumes",
			documents: []config.Document{v1alpha1Cfg, defaultEphemeral, encryptedState},
		},
		{
			name:      "v1alpha1 with no-conflict volumes",
			documents: []config.Document{v1alpha1Cfg, encryptedState},
		},
		{
			name:      "no v1alpha1",
			documents: []config.Document{encryptedEphemeral, encryptedState},
		},
		{
			name:          "conflict on ephemeral encryption",
			documents:     []config.Document{v1alpha1Cfg, encryptedEphemeral},
			expectedError: "1 error occurred:\n\t* system disk encryption for \"EPHEMERAL\" is configured in both v1alpha1.Config and VolumeConfig\n\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(tt.documents...)
			require.NoError(t, err)

			warnings, err := ctr.Validate(validationMode{})

			if tt.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.expectedError)
			}

			require.Equal(t, tt.expectedWarnings, warnings)
		})
	}
}

func TestValidateContainer(t *testing.T) {
	t.Parallel()

	sideroLinkCfg := siderolink.NewConfigV1Alpha1()
	sideroLinkCfg.APIUrlConfig.URL = must.Value(url.Parse("https://siderolink.api/?jointoken=secret&user=alice"))(t)

	v1alpha1Cfg := &v1alpha1.Config{
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{
				Endpoint: &v1alpha1.Endpoint{
					URL: must.Value(url.Parse("https://localhost:6443"))(t),
				},
			},
		},
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "worker",
			MachineCA: &x509.PEMEncodedCertificateAndKey{
				Crt: []byte("cert"),
			},
		},
	}

	v1alpha1CfgHostDNS := v1alpha1Cfg.DeepCopy()
	v1alpha1CfgHostDNS.MachineConfig.MachineFeatures = &v1alpha1.FeaturesConfig{
		HostDNSSupport: &v1alpha1.HostDNSConfig{ //nolint:staticcheck // testing legacy features
			HostDNSConfigEnabled:        new(true),
			HostDNSForwardKubeDNSToHost: new(true),
		},
	}

	resolverConfig := network.NewResolverConfigV1Alpha1()
	resolverConfig.ResolverNameservers = []network.NameserverConfig{
		{
			Address: network.Addr{Addr: netip.MustParseAddr("1.1.1.1")},
		},
	}

	hostDNSResolverConfig := network.NewResolverConfigV1Alpha1()
	hostDNSResolverConfig.ResolverHostDNS = network.HostDNSConfig{
		HostDNSEnabled:              new(true),
		HostDNSForwardKubeDNSToHost: new(true),
	}

	for _, tt := range []struct {
		name        string
		documents   []config.Document
		inContainer bool

		expectedError string
	}{
		{
			name: "empty !container",
		},
		{
			name:        "empty container",
			inContainer: true,

			expectedError: "1 error occurred:\n\t* hostDNS config is required in container mode\n\n",
		},
		{
			name:        "empty v1alpha1 container",
			documents:   []config.Document{v1alpha1Cfg},
			inContainer: true,

			expectedError: "1 error occurred:\n\t* hostDNS config is required in container mode\n\n",
		},
		{
			name:        "just resolver in container",
			documents:   []config.Document{resolverConfig},
			inContainer: true,

			expectedError: "1 error occurred:\n\t* hostDNS config is required in container mode\n\n",
		},
		{
			name:        "hostDNS v1alpha1 container",
			documents:   []config.Document{v1alpha1CfgHostDNS},
			inContainer: true,
		},
		{
			name:        "hostDNS v1alpha1 container plus multi-doc",
			documents:   []config.Document{v1alpha1CfgHostDNS, resolverConfig},
			inContainer: true,
		},
		{
			name:        "just multi-doc with hostDNS",
			documents:   []config.Document{hostDNSResolverConfig},
			inContainer: true,
		},
		{
			name:        "multi-doc with hostDNS and v1alpha1",
			documents:   []config.Document{hostDNSResolverConfig, v1alpha1Cfg},
			inContainer: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(tt.documents...)
			require.NoError(t, err)

			warnings, err := ctr.Validate(validationMode{inContainer: tt.inContainer})

			if tt.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.expectedError)
			}

			require.Nil(t, warnings)
		})
	}
}

type validationMode struct {
	inContainer bool
}

func (validationMode) String() string {
	return ""
}

func (validationMode) RequiresInstall() bool {
	return false
}

func (v validationMode) InContainer() bool {
	return v.inContainer
}
