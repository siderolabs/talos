// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	_ "embed"
	"net/url"
	"testing"

	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed testdata/discoveryserviceconfig.yaml
var expectedDiscoveryServiceConfigDocument []byte

//go:embed testdata/discoveryserviceconfig-multiple.yaml
var expectedMultipleDiscoveryServiceConfigDocuments []byte

func TestDiscoveryServiceConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := cluster.NewDiscoveryServiceConfigV1Alpha1("primary", must.Value(url.Parse("https://discovery.talos.dev/"))(t))

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedDiscoveryServiceConfigDocument, marshaled)
}

func TestDiscoveryServiceConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedDiscoveryServiceConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &cluster.DiscoveryServiceConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       cluster.DiscoveryServiceKind,
		},
		MetaName: "primary",
		EndpointURL: meta.URL{
			URL: must.Value(url.Parse("https://discovery.talos.dev/"))(t),
		},
	}, docs[0])
}

func TestMultipleDiscoveryServiceConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedMultipleDiscoveryServiceConfigDocuments)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 3)

	expectedConfigs := []struct {
		name     string
		endpoint string
	}{
		{"primary", "https://discovery.talos.dev/"},
		{"secondary", "https://discovery-secondary.talos.dev/path"},
		{"grpc-endpoint", "grpc://discovery-grpc.talos.dev:6443"},
	}

	for i, expected := range expectedConfigs {
		doc := docs[i]
		require.IsType(t, &cluster.DiscoveryServiceConfigV1Alpha1{}, doc)
		cfg := doc.(*cluster.DiscoveryServiceConfigV1Alpha1)

		assert.Equal(t, expected.name, cfg.MetaName)
		assert.Equal(t, cluster.DiscoveryServiceKind, cfg.Meta.MetaKind)
		assert.Equal(t, "v1alpha1", cfg.Meta.MetaAPIVersion)
		assert.Equal(t, expected.endpoint, cfg.EndpointURL.String())
	}
}

func TestDiscoveryServiceConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *cluster.DiscoveryServiceConfigV1Alpha1

		expectedError string
	}{
		{
			name: "valid http",
			cfg: func() *cluster.DiscoveryServiceConfigV1Alpha1 {
				return cluster.NewDiscoveryServiceConfigV1Alpha1("primary", must.Value(url.Parse("http://discovery.talos.dev/"))(t))
			},
		},
		{
			name: "valid https",
			cfg: func() *cluster.DiscoveryServiceConfigV1Alpha1 {
				return cluster.NewDiscoveryServiceConfigV1Alpha1("primary", must.Value(url.Parse("https://discovery.talos.dev/"))(t))
			},
		},
		{
			name: "valid grpc",
			cfg: func() *cluster.DiscoveryServiceConfigV1Alpha1 {
				return cluster.NewDiscoveryServiceConfigV1Alpha1("primary", must.Value(url.Parse("grpc://discovery.talos.dev/"))(t))
			},
		},
		{
			name: "missing name",
			cfg: func() *cluster.DiscoveryServiceConfigV1Alpha1 {
				return cluster.NewDiscoveryServiceConfigV1Alpha1("", must.Value(url.Parse("https://discovery.talos.dev/"))(t))
			},
			expectedError: "name is required",
		},
		{
			name: "missing endpoint",
			cfg: func() *cluster.DiscoveryServiceConfigV1Alpha1 {
				return cluster.NewDiscoveryServiceConfigV1Alpha1("primary", nil)
			},
			expectedError: "endpoint is required",
		},
		{
			name: "invalid endpoint scheme",
			cfg: func() *cluster.DiscoveryServiceConfigV1Alpha1 {
				return cluster.NewDiscoveryServiceConfigV1Alpha1("primary", must.Value(url.Parse("ftp://discovery.talos.dev/"))(t))
			},
			expectedError: "endpoint scheme must be http://, https:// or grpc://",
		},
		{
			name: "valid endpoint with root path",
			cfg: func() *cluster.DiscoveryServiceConfigV1Alpha1 {
				return cluster.NewDiscoveryServiceConfigV1Alpha1("primary", must.Value(url.Parse("https://discovery.talos.dev/"))(t))
			},
		},
		{
			name: "valid endpoint with non-root path",
			cfg: func() *cluster.DiscoveryServiceConfigV1Alpha1 {
				return cluster.NewDiscoveryServiceConfigV1Alpha1("primary", must.Value(url.Parse("https://discovery.talos.dev/some/path"))(t))
			},
		},
		{
			name: "missing host",
			cfg: func() *cluster.DiscoveryServiceConfigV1Alpha1 {
				return cluster.NewDiscoveryServiceConfigV1Alpha1("primary", must.Value(url.Parse("https://"))(t))
			},
			expectedError: "endpoint host is required",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})
			assert.Nil(t, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDiscoveryServiceConfigV1Alpha1ConflictValidate(t *testing.T) {
	t.Parallel()

	cfg := cluster.NewDiscoveryServiceConfigV1Alpha1("primary", must.Value(url.Parse("https://discovery.talos.dev/"))(t))

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
			name:        "cluster config without discovery block",
			v1alpha1Cfg: &v1alpha1.Config{ClusterConfig: &v1alpha1.ClusterConfig{}},
		},
		{
			name: "legacy discovery block present",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // testing legacy config conflict
						DiscoveryEnabled: new(true),
					},
				},
			},
			expectedError: "discovery service is already configured in .cluster.discovery of the v1alpha1 config",
		},
		{
			// even an empty/disabled .cluster.discovery block conflicts: presence is what matters
			name: "legacy discovery block present but disabled",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // testing legacy config conflict
						DiscoveryEnabled: new(false),
					},
				},
			},
			expectedError: "discovery service is already configured in .cluster.discovery of the v1alpha1 config",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := cfg.V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
