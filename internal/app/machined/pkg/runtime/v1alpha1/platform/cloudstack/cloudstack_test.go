// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cloudstack_test

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/cloudstack"
)

//go:embed testdata/metadata.json
var rawMetadata []byte

//go:embed testdata/expected.yaml
var expectedNetworkConfig string

func TestEmpty(t *testing.T) {
	p := &cloudstack.Cloudstack{}

	var m cloudstack.MetadataConfig

	require.NoError(t, json.Unmarshal(rawMetadata, &m))

	networkConfig, err := p.ParseMetadata(&m)
	require.NoError(t, err)

	marshaled, err := yaml.Marshal(networkConfig)
	require.NoError(t, err)

	assert.Equal(t, expectedNetworkConfig, string(marshaled))
}
