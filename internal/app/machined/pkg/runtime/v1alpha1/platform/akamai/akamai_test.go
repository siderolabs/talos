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

//go:embed testdata/instance-no-tags.json
var rawInstanceNoTags []byte

//go:embed testdata/instance-with-tags.json
var rawInstanceWithTags []byte

//go:embed testdata/network.json
var rawNetwork []byte

//go:embed testdata/expected-no-tags.yaml
var expectedNoTags string

//go:embed testdata/expected-with-tags.yaml
var expectedWithTags string

func TestParseMetadata(t *testing.T) {
	for _, tt := range []struct {
		name     string
		instance []byte
		expected string
	}{
		{
			name:     "no tags",
			instance: rawInstanceNoTags,
			expected: expectedNoTags,
		},
		{
			name:     "with tags",
			instance: rawInstanceWithTags,
			expected: expectedWithTags,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := &akamai.Akamai{}

			var metadata akametadata.InstanceData

			var interfaceConfig akametadata.NetworkData

			require.NoError(t, json.Unmarshal(tt.instance, &metadata))
			require.NoError(t, json.Unmarshal(rawNetwork, &interfaceConfig))

			networkConfig, err := p.ParseMetadata(&metadata, &interfaceConfig)
			require.NoError(t, err)

			marshaled, err := yaml.Marshal(networkConfig)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, string(marshaled))
		})
	}
}

func TestConvertTagsFromAkamai(t *testing.T) {
	for _, tt := range []struct {
		name     string
		input    []string
		expected map[string]string
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: nil,
		},
		{
			name:  "single tag",
			input: []string{"tag1"},
			expected: map[string]string{
				"tag1": "",
			},
		},
		{
			name:  "multiple tags",
			input: []string{"tag1", "tag2", "tag3"},
			expected: map[string]string{
				"tag1": "",
				"tag2": "",
				"tag3": "",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, akamai.ConvertTagsFromAkamai(tt.input))
		})
	}
}
