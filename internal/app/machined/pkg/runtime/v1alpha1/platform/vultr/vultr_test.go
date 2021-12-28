// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vultr_test

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vultr/metadata"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/vultr"
)

//go:embed testdata/metadata.json
var rawMetadata []byte

//go:embed testdata/expected.yaml
var expectedNetworkConfig string

func TestParseMetadata(t *testing.T) {
	p := &vultr.Vultr{}

	var m metadata.MetaData

	require.NoError(t, json.Unmarshal(rawMetadata, &m))

	networkConfig, err := p.ParseMetadata(&m, []byte("1.2.3.4"))
	require.NoError(t, err)

	marshaled, err := yaml.Marshal(networkConfig)
	require.NoError(t, err)

	assert.Equal(t, expectedNetworkConfig, string(marshaled))
}
