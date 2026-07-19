// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestCRIConfigBridge(t *testing.T) {
	t.Parallel()

	cfg := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineFiles: []*v1alpha1.MachineFile{ //nolint:staticcheck // test deprecated compatibility
				{
					FilePath:    filepath.Join("/etc", constants.CRICustomizationConfigPart),
					FileContent: "legacy customization",
				},
			},
			MachineBaseRuntimeSpecOverrides: meta.Unstructured{ //nolint:staticcheck // test deprecated compatibility
				Object: map[string]any{"process": map[string]any{"cwd": "/legacy"}},
			},
		},
	}

	customizations := cfg.CRICustomizationConfigs()
	require.Len(t, customizations, 1)
	assert.Equal(t, config.LegacyCRICustomizationConfigName, customizations[0].Name())
	assert.Equal(t, "legacy customization", customizations[0].Content())

	require.NotNil(t, cfg.CRIBaseRuntimeSpecConfig())
	assert.Equal(t, cfg.MachineConfig.MachineBaseRuntimeSpecOverrides.Object, cfg.CRIBaseRuntimeSpecConfig().Overrides()) //nolint:staticcheck // test deprecated compatibility
}

func TestEmptyCRIConfigBridge(t *testing.T) {
	t.Parallel()

	cfg := &v1alpha1.Config{}

	assert.Empty(t, cfg.CRICustomizationConfigs())
	assert.Nil(t, cfg.CRIBaseRuntimeSpecConfig())
}
