// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package aws_test

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/aws"
)

//go:embed testdata/metadata.json
var rawMetadata []byte

//go:embed testdata/metadata-v6.json
var rawMetadataV6 []byte

//go:embed testdata/metadata-v6only.json
var rawMetadataV6Only []byte

//go:embed testdata/expected.yaml
var expectedNetworkConfig string

//go:embed testdata/expected-v6.yaml
var expectedNetworkConfigV6 string

//go:embed testdata/expected-v6only.yaml
var expectedNetworkConfigV6Only string

func TestParseMetadata(t *testing.T) {
	for _, tt := range []struct {
		name     string
		raw      []byte
		expected string
	}{
		{
			name:     "IPv4 only",
			raw:      rawMetadata,
			expected: expectedNetworkConfig,
		},
		{
			name:     "dual stack",
			raw:      rawMetadataV6,
			expected: expectedNetworkConfigV6,
		},
		{
			name:     "IPv6 only",
			raw:      rawMetadataV6Only,
			expected: expectedNetworkConfigV6Only,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := &aws.AWS{}

			var metadata aws.MetadataConfig

			require.NoError(t, json.Unmarshal(tt.raw, &metadata))

			networkConfig, err := p.ParseMetadata(&metadata)
			require.NoError(t, err)

			marshaled, err := yaml.Marshal(networkConfig)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, string(marshaled))
		})
	}
}
