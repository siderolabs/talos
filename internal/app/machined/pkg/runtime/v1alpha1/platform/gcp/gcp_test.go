// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gcp_test

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/gcp"
)

//go:embed testdata/metadata.json
var rawMetadata []byte

//go:embed testdata/interfaces.json
var rawInterfaces []byte

//go:embed testdata/expected.yaml
var expectedNetworkConfig string

func TestParseMetadata(t *testing.T) {
	p := &gcp.GCP{}

	var (
		metadata   gcp.MetadataConfig
		interfaces []gcp.NetworkInterfaceConfig
	)

	require.NoError(t, json.Unmarshal(rawMetadata, &metadata))
	require.NoError(t, json.Unmarshal(rawInterfaces, &interfaces))

	networkConfig, err := p.ParseMetadata(&metadata, interfaces)
	require.NoError(t, err)

	marshaled, err := yaml.Marshal(networkConfig)
	require.NoError(t, err)

	assert.Equal(t, expectedNetworkConfig, string(marshaled))
}
