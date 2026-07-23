// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	_ "embed"
	"net/url"
	"testing"

	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed testdata/kubeclusterconfig.yaml
var expectedKubeClusterConfigDocument []byte

func kubeClusterConfig(name, endpoint string) *k8s.KubeClusterConfigV1Alpha1 {
	cfg := k8s.NewKubeClusterConfigV1Alpha1()
	cfg.ClusterNameConfig = name

	if endpoint != "" {
		cfg.ClusterEndpointConfig = meta.URL{URL: ensure.Value(url.Parse(endpoint))}
	}

	return cfg
}

func TestKubeClusterConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := kubeClusterConfig("example-cluster", "https://example.com:6443/")

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeClusterConfigDocument, marshaled)
}

func TestKubeClusterConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeClusterConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeClusterConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeClusterConfig,
		},
		ClusterNameConfig: "example-cluster",
		ClusterEndpointConfig: meta.URL{
			URL: must.Value(url.Parse("https://example.com:6443/"))(t),
		},
	}, docs[0])
}

func TestKubeClusterConfigAccessors(t *testing.T) {
	t.Parallel()

	cfg := kubeClusterConfig("example-cluster", "https://example.com:6443/")

	assert.Equal(t, "example-cluster", cfg.ClusterName())
	assert.Equal(t, "https://example.com:6443/", cfg.ClusterEndpoint().String())
}

func TestKubeClusterConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeClusterConfigV1Alpha1

		expectedError string
	}{
		{
			name: "valid",
			cfg: func() *k8s.KubeClusterConfigV1Alpha1 {
				return kubeClusterConfig("example-cluster", "https://example.com:6443/")
			},
		},
		{
			name: "missing cluster name",
			cfg: func() *k8s.KubeClusterConfigV1Alpha1 {
				return kubeClusterConfig("", "https://example.com:6443/")
			},
			expectedError: "clusterName must be specified",
		},
		{
			name: "missing endpoint",
			cfg: func() *k8s.KubeClusterConfigV1Alpha1 {
				return kubeClusterConfig("example-cluster", "")
			},
			expectedError: "endpoint must be specified",
		},
		{
			name: "invalid endpoint",
			cfg: func() *k8s.KubeClusterConfigV1Alpha1 {
				return kubeClusterConfig("example-cluster", "https://:6443/")
			},
			expectedError: "cluster endpoint is invalid: hostname must not be blank",
		},
		{
			name: "both missing",
			cfg: func() *k8s.KubeClusterConfigV1Alpha1 {
				return kubeClusterConfig("", "")
			},
			expectedError: "clusterName must be specified\nendpoint must be specified",
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

func TestKubeClusterConfigV1Alpha1ConflictValidate(t *testing.T) {
	t.Parallel()

	cfg := kubeClusterConfig("example-cluster", "https://example.com:6443/")

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
			name:        "cluster config without name or endpoint",
			v1alpha1Cfg: &v1alpha1.Config{ClusterConfig: &v1alpha1.ClusterConfig{}},
		},
		{
			name: "legacy cluster name present",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterName: "legacy-cluster", //nolint:staticcheck // testing legacy config conflict
				},
			},
			expectedError: "cluster name is already set in the v1alpha1 config (.cluster.clusterName). " +
				"Please remove it and use only the new KubeClusterConfig document to avoid conflicts",
		},
		{
			name: "legacy cluster endpoint present",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{ //nolint:staticcheck // testing legacy config conflict
						Endpoint: &v1alpha1.Endpoint{URL: must.Value(url.Parse("https://legacy.example.com:6443/"))(t)},
					},
				},
			},
			expectedError: "cluster endpoint is already set in the v1alpha1 config (.cluster.controlPlane.endpoint). " +
				"Please remove it and use only the new KubeClusterConfig document to avoid conflicts",
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
