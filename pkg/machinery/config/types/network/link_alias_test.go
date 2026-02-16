// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

//go:embed testdata/linkaliasconfig.yaml
var expectedLinkAliasConfigDocument []byte

func TestLinkAliasConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewLinkAliasConfigV1Alpha1("net0")
	cfg.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`mac(link.permanent_addr) == "00:1a:2b:3c:4d:5e"`, celenv.LinkLocator()))

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedLinkAliasConfigDocument, marshaled)
}

func TestLinkAliasConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedLinkAliasConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	c := &network.LinkAliasConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.LinkAliasKind,
		},
		MetaName: "net0",
	}
	require.NoError(t, c.Selector.Match.UnmarshalText([]byte(`mac(link.permanent_addr) == "00:1a:2b:3c:4d:5e"`)))

	assert.Equal(t, c, docs[0])
}

func TestLinkAliasValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.LinkAliasConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.LinkAliasConfigV1Alpha1 {
				return network.NewLinkAliasConfigV1Alpha1("")
			},

			expectedError: "name must be specified\nlink selector is required",
		},
		{
			name: "no disk selector",
			cfg: func() *network.LinkAliasConfigV1Alpha1 {
				return network.NewLinkAliasConfigV1Alpha1("int0")
			},

			expectedError: "link selector is required",
		},
		{
			name: "invalid disk selector",
			cfg: func() *network.LinkAliasConfigV1Alpha1 {
				c := network.NewLinkAliasConfigV1Alpha1("int0")
				require.NoError(t, c.Selector.Match.UnmarshalText([]byte(`disk.size > 120`)))

				return c
			},

			expectedError: "link selector is invalid: ERROR: <input>:1:1: undeclared reference to 'disk' (in container '')\n | disk.size > 120\n | ^",
		},
		{
			name: "valid",
			cfg: func() *network.LinkAliasConfigV1Alpha1 {
				c := network.NewLinkAliasConfigV1Alpha1("int0")
				require.NoError(t, c.Selector.Match.UnmarshalText([]byte(`mac(link.permanent_addr) == "00:1a:2b:3c:4d:5e"`)))

				return c
			},
		},
		{
			name: "valid pattern name",
			cfg: func() *network.LinkAliasConfigV1Alpha1 {
				c := network.NewLinkAliasConfigV1Alpha1("net%d")
				require.NoError(t, c.Selector.Match.UnmarshalText([]byte(`link.type == 1`)))

				return c
			},
		},
		{
			name: "invalid pattern name with padding",
			cfg: func() *network.LinkAliasConfigV1Alpha1 {
				c := network.NewLinkAliasConfigV1Alpha1("net%02d")
				require.NoError(t, c.Selector.Match.UnmarshalText([]byte(`link.type == 1`)))

				return c
			},

			expectedError: "name \"net%02d\" contains an invalid format verb, use %d suffix",
		},
		{
			name: "invalid pattern name with multiple verbs",
			cfg: func() *network.LinkAliasConfigV1Alpha1 {
				c := network.NewLinkAliasConfigV1Alpha1("net%d-port%d")
				require.NoError(t, c.Selector.Match.UnmarshalText([]byte(`link.type == 1`)))

				return c
			},

			expectedError: "name \"net%d-port%d\" contains an invalid format verb, use %d suffix",
		},
		{
			name: "invalid format verb",
			cfg: func() *network.LinkAliasConfigV1Alpha1 {
				c := network.NewLinkAliasConfigV1Alpha1("net%s")
				require.NoError(t, c.Selector.Match.UnmarshalText([]byte(`link.type == 1`)))

				return c
			},

			expectedError: `name "net%s" contains an invalid format verb, use %d suffix`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
