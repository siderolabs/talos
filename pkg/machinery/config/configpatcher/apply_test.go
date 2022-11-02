// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configpatcher_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
)

//go:embed testdata/apply/config.yaml
var config []byte

//go:embed testdata/apply/expected.yaml
var expected []byte

func TestApply(t *testing.T) {
	patches, err := configpatcher.LoadPatches([]string{
		"@testdata/apply/strategic1.yaml",
		"@testdata/apply/jsonpatch1.yaml",
		"@testdata/apply/jsonpatch2.yaml",
		"@testdata/apply/strategic2.yaml",
	})
	require.NoError(t, err)

	cfg, err := configloader.NewFromBytes(config)
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		input configpatcher.Input
	}{
		{
			name:  "WithConfig",
			input: configpatcher.WithConfig(cfg),
		},
		{
			name:  "WithBytes",
			input: configpatcher.WithBytes(config),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := configpatcher.Apply(tt.input, patches)
			require.NoError(t, err)

			bytes, err := out.Bytes()
			require.NoError(t, err)

			assert.Equal(t, expected, bytes)
		})
	}
}
