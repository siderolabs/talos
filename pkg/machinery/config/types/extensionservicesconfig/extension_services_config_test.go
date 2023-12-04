// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensionservicesconfig_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/extensionservicesconfig"
)

//go:embed testdata/extension_service_config.yaml
var expectedExtensionServicesConfigDocument []byte

func TestExtensionServicesConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := extensionservicesconfig.NewExtensionServicesConfigV1Alpha1()
	cfg.Config = []extensionservicesconfig.ExtensionServiceConfig{
		{
			ExtensionName: "foo",
			ExtensionServiceConfigFiles: []extensionservicesconfig.ExtensionServiceConfigFile{
				{
					ExtensionContent:   "hello",
					ExtensionMountPath: "/etc/foo",
				},
			},
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedExtensionServicesConfigDocument, marshaled)
}
