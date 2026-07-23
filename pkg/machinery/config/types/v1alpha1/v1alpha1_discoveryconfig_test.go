// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// TestDiscoveryServiceConfigsLegacyAdapter covers the (*Config).DiscoveryServiceConfigs() adapter
// that surfaces the deprecated .cluster.discovery block via the new config.DiscoveryServiceConfig interface.
func TestDiscoveryServiceConfigsLegacyAdapter(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  *v1alpha1.Config

		// expected endpoint of the single surfaced config, empty string means no config surfaced
		expectedEndpoint string
	}{
		{
			name: "nil cluster config",
			cfg:  &v1alpha1.Config{},
		},
		{
			name: "cluster config without discovery block",
			cfg:  &v1alpha1.Config{ClusterConfig: &v1alpha1.ClusterConfig{}},
		},
		{
			name: "discovery disabled",
			cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // testing legacy config
						DiscoveryEnabled: new(false),
						DiscoveryRegistries: v1alpha1.DiscoveryRegistriesConfig{ //nolint:staticcheck // testing legacy config
							RegistryService: v1alpha1.RegistryServiceConfig{ //nolint:staticcheck // testing legacy config
								RegistryEndpoint: "https://custom.discovery.test/",
							},
						},
					},
				},
			},
		},
		{
			name: "discovery enabled but service registry disabled",
			cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // testing legacy config
						DiscoveryEnabled: new(true),
						DiscoveryRegistries: v1alpha1.DiscoveryRegistriesConfig{ //nolint:staticcheck // testing legacy config
							RegistryService: v1alpha1.RegistryServiceConfig{ //nolint:staticcheck // testing legacy config
								RegistryDisabled: new(true),
								RegistryEndpoint: "https://custom.discovery.test/",
							},
						},
					},
				},
			},
		},
		{
			name: "discovery enabled, custom service endpoint",
			cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // testing legacy config
						DiscoveryEnabled: new(true),
						DiscoveryRegistries: v1alpha1.DiscoveryRegistriesConfig{ //nolint:staticcheck // testing legacy config
							RegistryService: v1alpha1.RegistryServiceConfig{ //nolint:staticcheck // testing legacy config
								RegistryEndpoint: "https://custom.discovery.test/",
							},
						},
					},
				},
			},
			expectedEndpoint: "https://custom.discovery.test/",
		},
		{
			name: "discovery enabled, empty endpoint falls back to default",
			cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // testing legacy config
						DiscoveryEnabled: new(true),
					},
				},
			},
			expectedEndpoint: constants.DefaultDiscoveryServiceEndpoint,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := test.cfg.DiscoveryServiceConfigs()

			if test.expectedEndpoint == "" {
				assert.Empty(t, got)

				return
			}

			assert.Len(t, got, 1)
			assert.Equal(t, "legacy", got[0].Name())
			assert.Equal(t, test.expectedEndpoint, got[0].Endpoint().String())
		})
	}
}
