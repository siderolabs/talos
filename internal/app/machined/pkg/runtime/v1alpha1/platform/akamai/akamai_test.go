// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package akamai_test

import (
	_ "embed"
	"encoding/json"
	"testing"

	akametadata "github.com/linode/go-metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/akamai"
)

//go:embed testdata/instance.json
var rawMetadata []byte

//go:embed testdata/network.json

var rawNetwork []byte

//go:embed testdata/expected.yaml
var expectedNetworkConfig string

func TestParseMetadata(t *testing.T) {
	p := &akamai.Akamai{}

	var metadata akametadata.InstanceData

	var interfaceConfig akametadata.NetworkData

	require.NoError(t, json.Unmarshal(rawMetadata, &metadata))

	require.NoError(t, json.Unmarshal(rawNetwork, &interfaceConfig))

	networkConfig, err := p.ParseMetadata(&metadata, &interfaceConfig)
	require.NoError(t, err)

	marshaled, err := yaml.Marshal(networkConfig)
	require.NoError(t, err)

	assert.Equal(t, expectedNetworkConfig, string(marshaled))
}
