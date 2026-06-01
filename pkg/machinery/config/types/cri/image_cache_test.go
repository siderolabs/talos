// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed testdata/imagecacheconfig.yaml
var expectedImageCacheConfigDocument []byte

func TestImageCacheConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := cri.NewImageCacheConfigV1Alpha1()
	cfg.LocalConfig.ConfigEnabled = new(true)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedImageCacheConfigDocument, marshaled)
}

func TestImageCacheConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedImageCacheConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &cri.ImageCacheConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       cri.ImageCacheConfig,
		},
		LocalConfig: cri.LocalImageCacheConfig{
			ConfigEnabled: new(true),
		},
	}, docs[0])
}

func TestImageCacheConfigV1Alpha1Conflict(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		v1alpha1Cfg *v1alpha1.Config
		cfg         func() *cri.ImageCacheConfigV1Alpha1

		expectedError string
	}{
		{
			name:        "empty",
			v1alpha1Cfg: &v1alpha1.Config{},
			cfg:         cri.NewImageCacheConfigV1Alpha1,
		},
		{
			name: "v1alpha1 image cache enabled",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{
						ImageCacheSupport: &v1alpha1.ImageCacheConfig{
							CacheLocalEnabled: new(true),
						},
					},
				},
			},
			cfg: cri.NewImageCacheConfigV1Alpha1,

			expectedError: "image cache config is already set in v1alpha1 config",
		},
		{
			name: "v1alpha1 image cache disabled",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{
						ImageCacheSupport: &v1alpha1.ImageCacheConfig{
							CacheLocalEnabled: new(false),
						},
					},
				},
			},
			cfg: cri.NewImageCacheConfigV1Alpha1,

			expectedError: "image cache config is already set in v1alpha1 config",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.cfg().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
