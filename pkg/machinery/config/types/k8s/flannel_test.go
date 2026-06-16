// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	_ "embed"
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

//go:embed testdata/flannelconfig.yaml
var expectedKubeFlannelCNIConfigDocument []byte

func TestKubeFlannelCNIConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeFlannelCNIConfigV1Alpha1()
	cfg.FlannelBackendType = constants.FlannelDefaultBackend
	cfg.FlannelBackendPort = constants.FlannelDefaultBackendPort
	cfg.FlannelBackendMTU = 1420
	cfg.FlannelExtraArgs = []string{"--iface-can-reach=10.0.0.1"}
	cfg.FlannelKubeNetworkPoliciesEnabled = new(true)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeFlannelCNIConfigDocument, marshaled)
}

func TestKubeFlannelCNIConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeFlannelCNIConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeFlannelCNIConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeFlannelCNIConfig,
		},
		FlannelBackendType:                constants.FlannelDefaultBackend,
		FlannelBackendPort:                constants.FlannelDefaultBackendPort,
		FlannelBackendMTU:                 1420,
		FlannelExtraArgs:                  []string{"--iface-can-reach=10.0.0.1"},
		FlannelKubeNetworkPoliciesEnabled: new(true),
	}, docs[0])
}

//nolint:dupl
func TestKubeFlannelCNIConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeFlannelCNIConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeFlannelCNIConfigV1Alpha1,

			expectedError: "flannel backend type must be specified",
		},
		{
			name: "invalid resources",
			cfg: func() *k8s.KubeFlannelCNIConfigV1Alpha1 {
				cfg := k8s.NewKubeFlannelCNIConfigV1Alpha1()
				cfg.FlannelBackendType = constants.FlannelDefaultBackend
				cfg.FlannelResources = k8s.ResourcesConfig{
					Requests: meta.Unstructured{
						Object: map[string]any{
							"invalid": "1",
						},
					},
				}

				return cfg
			},

			expectedError: "unsupported pod resource \"invalid\"",
		},
		{
			name: "valid config",
			cfg: func() *k8s.KubeFlannelCNIConfigV1Alpha1 {
				cfg := k8s.NewKubeFlannelCNIConfigV1Alpha1()
				cfg.FlannelBackendType = constants.FlannelDefaultBackend
				cfg.FlannelBackendPort = constants.FlannelDefaultBackendPort

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
func TestKubeFlannelCNIConfigV1Alpha1Validate(t *testing.T) {
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

			expectedError: "cluster network config in v1alpha1 config (.machine.cluster.network) can't be used with KubeFlannelCNIConfig document, please remove it to avoid conflicts",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := k8s.NewKubeFlannelCNIConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
