// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	_ "embed"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//go:embed testdata/networkconfig.yaml
var expectedKubeNetworkConfigDocument []byte

func TestKubeNetworkConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeNetworkConfigV1Alpha1()
	cfg.NetworkDNSDomain = constants.DefaultDNSDomain
	cfg.NetworkPodSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodNet)},
	}
	cfg.NetworkServiceSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceNet)},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeNetworkConfigDocument, marshaled)
}

func TestKubeNetworkConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeNetworkConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeNetworkConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeNetworkConfig,
		},
		NetworkDNSDomain: constants.DefaultDNSDomain,
		NetworkPodSubnets: []meta.Prefix{
			{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodNet)},
		},
		NetworkServiceSubnets: []meta.Prefix{
			{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceNet)},
		},
	}, docs[0])
}

func TestKubeNetworkConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeNetworkConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeNetworkConfigV1Alpha1,

			expectedError: "pod subnets: at least one subnets must be specified\nservice subnets: at least one subnets must be specified",
		},
		{
			name: "double v4",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodNet)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodNet)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceNet)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceNet)},
				}

				return cfg
			},

			expectedError: "pod subnets: at most one IPv4 and one IPv6 subnet can be specified for " +
				"dual-stack clusters\nservice subnets: at most one IPv4 and one IPv6 subnet can be specified for dual-stack clusters",
		},
		{
			name: "length mismatch",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodNet)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodNet)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceNet)},
				}

				return cfg
			},

			expectedError: "the number of pod subnets must match the number of service subnets",
		},
		{
			name: "invalid subnet",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix("192.168.1.1/24")},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceNet)},
				}

				return cfg
			},

			expectedError: "pod subnets: invalid subnet: 192.168.1.1/24 is not a valid CIDR",
		},
		{
			name: "v4 only",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodNet)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceNet)},
				}

				return cfg
			},
		},
		{
			name: "v6 only",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodNet)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceNet)},
				}

				return cfg
			},
		},
		{
			name: "dual stack",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodNet)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodNet)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceNet)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceNet)},
				}

				return cfg
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

//nolint:dupl
func TestKubeNetworkConfigV1Alpha1Validate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		v1alpha1Cfg *v1alpha1.Config

		expectedError string
	}{
		{
			name:        "empty",
			v1alpha1Cfg: &v1alpha1.Config{},
		},
		{
			name: "v1alpha1 with cluster network config set",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterNetwork: &v1alpha1.ClusterNetworkConfig{}, //nolint:staticcheck // testing deprecated field
				},
			},

			expectedError: "cluster network config in v1alpha1 config (.machine.cluster.network) can't be used with KubeNetworkConfig document, please remove it to avoid conflicts",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := k8s.NewKubeNetworkConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
