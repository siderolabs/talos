// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nocloud_test

import (
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/nocloud"
)

//go:embed testdata/metadata-v1.yaml
var rawMetadataV1 []byte

//go:embed testdata/metadata-v2.yaml
var rawMetadataV2 []byte

//go:embed testdata/expected-v1.yaml
var expectedNetworkConfigV1 string

//go:embed testdata/expected-v2.yaml
var expectedNetworkConfigV2 string

func TestParseMetadata(t *testing.T) {
	for _, tt := range []struct {
		name     string
		raw      []byte
		hostname string
		expected string
	}{
		{
			name:     "V1",
			raw:      rawMetadataV1,
			hostname: "talos",
			expected: expectedNetworkConfigV1,
		},
		{
			name:     "V2",
			raw:      rawMetadataV2,
			expected: expectedNetworkConfigV2,
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			n := &nocloud.Nocloud{}

			var m nocloud.NetworkConfig

			require.NoError(t, yaml.Unmarshal(tt.raw, &m))

			networkConfig, err := n.ParseMetadata(&m, tt.hostname)
			require.NoError(t, err)

			marshaled, err := yaml.Marshal(networkConfig)
			require.NoError(t, err)

			fmt.Print(string(marshaled))

			assert.Equal(t, tt.expected, string(marshaled))
		})
	}
}
