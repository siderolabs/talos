// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime/extensions"
)

//go:embed testdata/extension_service_config.yaml
var expectedExtensionServiceConfigDocument []byte

func TestExtensionServiceConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := extensions.NewServicesConfigV1Alpha1()
	cfg.ServiceName = "foo"
	cfg.ServiceConfigFiles = []extensions.ConfigFile{
		{
			ConfigFileContent:   "hello",
			ConfigFileMountPath: "/etc/foo",
		},
	}
	cfg.ServiceEnvironment = []string{"FOO=BAR"}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedExtensionServiceConfigDocument, marshaled)
}

func TestExtensionServiceConfigMerge(t *testing.T) {
	t.Parallel()

	cfgLeft := extensions.NewServicesConfigV1Alpha1()
	cfgLeft.ServiceName = "foo"
	cfgLeft.ServiceConfigFiles = []extensions.ConfigFile{
		{
			ConfigFileContent:   "hello",
			ConfigFileMountPath: "/etc/foo",
		},
	}

	cfgRight := cfgLeft.DeepCopy()
	cfgRight.ServiceConfigFiles[0].ConfigFileContent = "hello world"

	cfgRight.ServiceConfigFiles = append(cfgRight.ServiceConfigFiles,
		extensions.ConfigFile{
			ConfigFileContent:   "bar",
			ConfigFileMountPath: "/etc/bar",
		},
	)

	require.NoError(t, merge.Merge(cfgLeft, cfgRight))

	require.Len(t, cfgLeft.ConfigFiles(), 2)

	assert.Equal(t, "hello world", cfgLeft.ConfigFiles()[0].Content())
	assert.Equal(t, "bar", cfgLeft.ConfigFiles()[1].Content())
}
