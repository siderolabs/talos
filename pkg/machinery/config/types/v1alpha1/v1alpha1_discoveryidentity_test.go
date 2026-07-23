// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

// TestDiscoveryIdentityConfigLegacyAdapter covers the (*Config).DiscoveryIdentityConfig() adapter
// that surfaces the deprecated .cluster.id/.cluster.secret fields via the new
// config.DiscoveryIdentityConfig interface.
func TestDiscoveryIdentityConfigLegacyAdapter(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  *v1alpha1.Config

		expectNil      bool
		expectedID     string
		expectedSecret string
	}{
		{
			name:      "nil cluster config",
			cfg:       &v1alpha1.Config{},
			expectNil: true,
		},
		{
			name:      "cluster config without identity",
			cfg:       &v1alpha1.Config{ClusterConfig: &v1alpha1.ClusterConfig{}},
			expectNil: true,
		},
		{
			name: "cluster id and secret present",
			cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterID:     "cluster-id",     //nolint:staticcheck // testing legacy config
					ClusterSecret: "cluster-secret", //nolint:staticcheck // testing legacy config
				},
			},
			expectedID:     "cluster-id",
			expectedSecret: "cluster-secret",
		},
		{
			name: "only cluster id present",
			cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterID: "cluster-id", //nolint:staticcheck // testing legacy config
				},
			},
			expectedID:     "cluster-id",
			expectedSecret: "",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := test.cfg.DiscoveryIdentityConfig()

			if test.expectNil {
				assert.Nil(t, got)

				return
			}

			require.NotNil(t, got)
			assert.Equal(t, test.expectedID, got.ClusterID())
			assert.Equal(t, test.expectedSecret, got.ClusterSecret())
		})
	}
}
