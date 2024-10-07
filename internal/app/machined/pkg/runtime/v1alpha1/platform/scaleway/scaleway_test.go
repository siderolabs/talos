// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package scaleway_test

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/scaleway"
)

//go:embed testdata/metadata-v1.json
var rawMetadataV1 []byte

//go:embed testdata/metadata-v2.json
var rawMetadataV2 []byte

//go:embed testdata/metadata-v3.json
var rawMetadataV3 []byte

//go:embed testdata/expected-v1.yaml
var expectedNetworkConfigV1 string

//go:embed testdata/expected-v2.yaml
var expectedNetworkConfigV2 string

//go:embed testdata/expected-v3.yaml
var expectedNetworkConfigV3 string

func TestParseMetadata(t *testing.T) {
	p := &scaleway.Scaleway{}

	for _, tt := range []struct {
		name     string
		raw      []byte
		expected string
	}{
		{
			name:     "V1",
			raw:      rawMetadataV1,
			expected: expectedNetworkConfigV1,
		},
		{
			name:     "V2",
			raw:      rawMetadataV2,
			expected: expectedNetworkConfigV2,
		},
		{
			name:     "V3",
			raw:      rawMetadataV3,
			expected: expectedNetworkConfigV3,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var metadata instance.Metadata

			require.NoError(t, json.Unmarshal(tt.raw, &metadata))

			networkConfig, err := p.ParseMetadata(&metadata)
			require.NoError(t, err)

			marshaled, err := yaml.Marshal(networkConfig)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, string(marshaled))
		})
	}
}
