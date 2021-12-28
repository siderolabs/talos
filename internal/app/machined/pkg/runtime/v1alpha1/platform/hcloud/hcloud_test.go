// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hcloud_test

import (
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/hcloud"
)

//go:embed testdata/metadata.yaml
var rawMetadata []byte

//go:embed testdata/expected.yaml
var expectedNetworkConfig string

func TestParseMetadata(t *testing.T) {
	h := &hcloud.Hcloud{}

	var m hcloud.NetworkConfig

	require.NoError(t, yaml.Unmarshal(rawMetadata, &m))

	networkConfig, err := h.ParseMetadata(&m, []byte("some.fqdn"), []byte("1.2.3.4"))
	require.NoError(t, err)

	marshaled, err := yaml.Marshal(networkConfig)
	require.NoError(t, err)

	fmt.Print(string(marshaled))

	assert.Equal(t, expectedNetworkConfig, string(marshaled))
}
