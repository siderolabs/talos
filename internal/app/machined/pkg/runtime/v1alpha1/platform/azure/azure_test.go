// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package azure_test

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/azure"
)

//go:embed metadata.json
var rawMetadata []byte

//go:embed expected.yaml
var expectedNetworkConfig string

func TestParseMetadata(t *testing.T) {
	a := &azure.Azure{}

	var m []azure.NetworkConfig

	require.NoError(t, json.Unmarshal(rawMetadata, &m))

	networkConfig, err := a.ParseMetadata(m, []byte("some.fqdn"))
	require.NoError(t, err)

	marshaled, err := yaml.Marshal(networkConfig)
	require.NoError(t, err)

	assert.Equal(t, expectedNetworkConfig, string(marshaled))
}
