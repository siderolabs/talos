// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package alibabacloud_test

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/alibabacloud"
)

//go:embed testdata/metadata.json
var rawMetadata []byte

//go:embed testdata/expected.yaml
var expectedNetworkConfig string

func TestParseMetadata(t *testing.T) {
	var metadata alibabacloud.MetadataConfig
	require.NoError(t, json.Unmarshal(rawMetadata, &metadata))

	config, err := (&alibabacloud.Alibabacloud{}).ParseMetadata(&metadata)
	require.NoError(t, err)

	marshaled, err := yaml.Marshal(config)
	require.NoError(t, err)
	assert.Equal(t, expectedNetworkConfig, string(marshaled))
}
