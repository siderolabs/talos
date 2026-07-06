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
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
	}
	cfg.NetworkServiceSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
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
			{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
		},
		NetworkServiceSubnets: []meta.Prefix{
			{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
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

			expectedError: "pod subnets: at least one subnet must be specified\nservice subnets: at least one subnet must be specified",
		},
		{
			name: "double v4",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
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
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
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
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
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
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
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
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceCIDR)},
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
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceCIDR)},
				}

				return cfg
			},
		},
		{
			name: "service subnet too large",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix("10.0.0.0/10")},
				}

				return cfg
			},

			expectedError: "service subnets: invalid subnet: 10.0.0.0/10 is too large, it must be at least /12 (at most 20 host identifier bits)",
		},
		{
			name: "pod subnet too large for node mask v4",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix("2.0.0.0/7")},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
				}

				return cfg
			},

			expectedError: "pod subnets: invalid subnet: 2.0.0.0/7 is too large for the per-node pod CIDR mask size /24, the difference must be at most 16 bits",
		},
		{
			name: "pod subnet too large for node mask v6",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix("fc00:db8::/40")},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceCIDR)},
				}

				return cfg
			},

			expectedError: "pod subnets: invalid subnet: fc00:db8::/40 is too large for the per-node pod CIDR mask size /64, the difference must be at most 16 bits",
		},
		{
			name: "pod subnet smaller than node mask",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix("10.244.0.0/28")},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
				}

				return cfg
			},

			expectedError: "pod subnets: invalid subnet: 10.244.0.0/28 is smaller than the per-node pod CIDR mask size /24",
		},
		{
			name: "custom node mask makes pod subnet valid",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix("2.0.0.0/7")},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
				}
				cfg.NetworkNodeCIDRMaskSizeIPv4 = 20

				return cfg
			},
		},
		{
			name: "custom node mask makes pod subnet invalid v6",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceCIDR)},
				}
				cfg.NetworkNodeCIDRMaskSizeIPv6 = 80

				return cfg
			},

			expectedError: "pod subnets: invalid subnet: fc00:db8:10::/56 is too large for the per-node pod CIDR mask size /80, the difference must be at most 16 bits",
		},
		{
			name: "node mask ipv4 negative",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
				}
				cfg.NetworkNodeCIDRMaskSizeIPv4 = -1

				return cfg
			},

			expectedError: "nodeCIDRMaskSizeIPv4 must be between 1 and 32",
		},
		{
			name: "node mask ipv4 too large",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
				}
				cfg.NetworkNodeCIDRMaskSizeIPv4 = 33

				return cfg
			},

			expectedError: "nodeCIDRMaskSizeIPv4 must be between 1 and 32",
		},
		{
			name: "node mask ipv6 negative",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceCIDR)},
				}
				cfg.NetworkNodeCIDRMaskSizeIPv6 = -5

				return cfg
			},

			expectedError: "nodeCIDRMaskSizeIPv6 must be between 1 and 128",
		},
		{
			name: "node mask ipv6 too large",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceCIDR)},
				}
				cfg.NetworkNodeCIDRMaskSizeIPv6 = 129

				return cfg
			},

			expectedError: "nodeCIDRMaskSizeIPv6 must be between 1 and 128",
		},
		{
			name: "node mask ipv4 and ipv6 both out of range",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceCIDR)},
				}
				cfg.NetworkNodeCIDRMaskSizeIPv4 = -1
				cfg.NetworkNodeCIDRMaskSizeIPv6 = 200

				return cfg
			},

			expectedError: "nodeCIDRMaskSizeIPv4 must be between 1 and 32\nnodeCIDRMaskSizeIPv6 must be between 1 and 128",
		},
		{
			name: "node mask ipv4 boundary valid at 1",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix("0.0.0.0/1")},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
				}
				cfg.NetworkNodeCIDRMaskSizeIPv4 = 1

				return cfg
			},
		},
		{
			name: "node mask ipv4 boundary valid at 32",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
				}
				cfg.NetworkNodeCIDRMaskSizeIPv4 = 32

				return cfg
			},
		},
		{
			name: "node mask ipv6 boundary valid at 1",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix("::/1")},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceCIDR)},
				}
				cfg.NetworkNodeCIDRMaskSizeIPv6 = 1

				return cfg
			},
		},
		{
			name: "node mask ipv6 boundary valid at 128",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix("fc00:db8:10::/120")},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceCIDR)},
				}
				cfg.NetworkNodeCIDRMaskSizeIPv6 = 128

				return cfg
			},
		},
		{
			name: "node mask ipv4 and ipv6 explicitly unset",
			cfg: func() *k8s.KubeNetworkConfigV1Alpha1 {
				cfg := k8s.NewKubeNetworkConfigV1Alpha1()
				cfg.NetworkDNSDomain = constants.DefaultDNSDomain
				cfg.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodCIDR)},
				}
				cfg.NetworkServiceSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
					{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceCIDR)},
				}
				cfg.NetworkNodeCIDRMaskSizeIPv4 = 0
				cfg.NetworkNodeCIDRMaskSizeIPv6 = 0

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

			expectedError: "cluster network config is already set in the v1alpha1 config (.machine.cluster.network). Please remove it and use only the new KubeNetworkConfig document to avoid conflicts",
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
