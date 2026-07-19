// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed testdata/cribaseruntimespecconfig.yaml
var expectedCRIBaseRuntimeSpecConfigDocument []byte

func TestCRIBaseRuntimeSpecConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := cri.NewCRIBaseRuntimeSpecConfigV1Alpha1()
	cfg.OverridesConfig.Object = baseRuntimeSpecOverrides()

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	assert.Equal(t, expectedCRIBaseRuntimeSpecConfigDocument, marshaled)
}

func TestCRIBaseRuntimeSpecConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedCRIBaseRuntimeSpecConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &cri.CRIBaseRuntimeSpecConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       cri.CRIBaseRuntimeSpecConfigKind,
		},
		OverridesConfig: meta.Unstructured{Object: baseRuntimeSpecOverrides()},
	}, docs[0])
}

func TestCRIBaseRuntimeSpecConfigMergeIsIdempotent(t *testing.T) {
	t.Parallel()

	document := cri.NewCRIBaseRuntimeSpecConfigV1Alpha1()
	document.OverridesConfig.Object = baseRuntimeSpecOverrides()

	currentContainer, err := container.New(document)
	require.NoError(t, err)

	var current coreconfig.Provider = currentContainer

	patchProvider, err := container.New(document.DeepCopy())
	require.NoError(t, err)

	patch := configpatcher.NewStrategicMergePatch(patchProvider)
	current, err = configpatcher.StrategicMerge(current, patch)
	require.NoError(t, err)

	first, err := current.Bytes()
	require.NoError(t, err)

	current, err = configpatcher.StrategicMerge(current, patch)
	require.NoError(t, err)

	second, err := current.Bytes()
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.Equal(t, baseRuntimeSpecOverrides(), current.CRIBaseRuntimeSpecConfig().Overrides())
}

func TestCRIBaseRuntimeSpecConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		overrides     map[string]any
		expectedError string
	}{
		{
			name:      "valid",
			overrides: baseRuntimeSpecOverrides(),
		},
		{
			name: "invalid rlimits",
			overrides: map[string]any{
				"process": map[string]any{"rlimits": "invalid"},
			},
			expectedError: "failed to unmarshal base runtime spec overrides",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := cri.NewCRIBaseRuntimeSpecConfigV1Alpha1()
			cfg.OverridesConfig.Object = test.overrides

			warnings, err := cfg.Validate(validationMode{})
			assert.Empty(t, warnings)

			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.expectedError)
			}
		})
	}
}

func TestCRIBaseRuntimeSpecConfigV1Alpha1Conflict(t *testing.T) {
	t.Parallel()

	cfg := cri.NewCRIBaseRuntimeSpecConfigV1Alpha1()

	assert.NoError(t, cfg.V1Alpha1ConflictValidate(&v1alpha1.Config{}))
	assert.NoError(t, cfg.V1Alpha1ConflictValidate(&v1alpha1.Config{MachineConfig: &v1alpha1.MachineConfig{}}))

	err := cfg.V1Alpha1ConflictValidate(&v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineBaseRuntimeSpecOverrides: meta.Unstructured{Object: map[string]any{}}, //nolint:staticcheck // test deprecated compatibility
		},
	})
	assert.EqualError(t, err, "base runtime spec overrides are already set in v1alpha1 config")

	err = cfg.V1Alpha1ConflictValidate(&v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineBaseRuntimeSpecOverrides: meta.Unstructured{ //nolint:staticcheck // test deprecated compatibility
				Object: baseRuntimeSpecOverrides(),
			},
		},
	})
	assert.EqualError(t, err, "base runtime spec overrides are already set in v1alpha1 config")
}

func baseRuntimeSpecOverrides() map[string]any {
	return map[string]any{
		"process": map[string]any{
			"rlimits": []any{
				map[string]any{
					"type": "RLIMIT_NOFILE",
					"hard": 1024,
					"soft": 1024,
				},
			},
		},
	}
}
