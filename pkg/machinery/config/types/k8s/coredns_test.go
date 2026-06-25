// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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

//go:embed testdata/corednsconfig.yaml
var expectedKubeCoreDNSConfigDocument []byte

func TestKubeCoreDNSConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeCoreDNSConfigV1Alpha1()
	cfg.PodEnabled = new(true)
	cfg.PodImage = constants.CoreDNSImage + ":" + constants.DefaultCoreDNSVersion

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeCoreDNSConfigDocument, marshaled)
}

func TestKubeCoreDNSConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeCoreDNSConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeCoreDNSConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeCoreDNSConfig,
		},
		PodEnabled: new(true),
		PodImage:   constants.CoreDNSImage + ":" + constants.DefaultCoreDNSVersion,
	}, docs[0])
}

func TestKubeCoreDNSConfigDefaults(t *testing.T) {
	t.Parallel()

	// an empty document defaults to enabled with the default CoreDNS image.
	cfg := k8s.NewKubeCoreDNSConfigV1Alpha1()

	assert.True(t, cfg.Enabled())
	assert.Equal(t, constants.CoreDNSImage+":"+constants.DefaultCoreDNSVersion, cfg.Image())

	// explicit values override the defaults.
	cfg.PodEnabled = new(false)
	cfg.PodImage = "example.com/coredns/coredns:v0.0.1"

	assert.False(t, cfg.Enabled())
	assert.Equal(t, "example.com/coredns/coredns:v0.0.1", cfg.Image())
}

//nolint:dupl
func TestKubeCoreDNSConfigV1Alpha1Validate(t *testing.T) {
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
			name: "v1alpha1 with cluster coredns config set",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					CoreDNSConfig: &v1alpha1.CoreDNS{}, //nolint:staticcheck // testing deprecated field
				},
			},

			expectedError: "CoreDNS config is already set in v1alpha1 config (.cluster.coreDNS)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := k8s.NewKubeCoreDNSConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
