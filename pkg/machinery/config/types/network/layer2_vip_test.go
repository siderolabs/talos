// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

//go:embed testdata/layer2vipconfig.yaml
var expectedLayer2VIPConfigDocument []byte

func TestLayer2VIPConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewLayer2VIPConfigV1Alpha1("1.2.3.4")
	cfg.LinkName = "net0"

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedLayer2VIPConfigDocument, marshaled)
}

func TestLayer2VIPConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedLayer2VIPConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	c := &network.Layer2VIPConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.Layer2VIPKind,
		},
		MetaName: "1.2.3.4",
		LinkName: "net0",
	}

	assert.Equal(t, c, docs[0])
}

func TestLayer2VIPValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.Layer2VIPConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.Layer2VIPConfigV1Alpha1 {
				return network.NewLayer2VIPConfigV1Alpha1("")
			},

			expectedError: "name must be specified\nlink must be specified",
		},
		{
			name: "no link name",
			cfg: func() *network.Layer2VIPConfigV1Alpha1 {
				return network.NewLayer2VIPConfigV1Alpha1("1.1.1.1")
			},

			expectedError: "link must be specified",
		},
		{
			name: "invalid IP",
			cfg: func() *network.Layer2VIPConfigV1Alpha1 {
				c := network.NewLayer2VIPConfigV1Alpha1("net4")
				c.LinkName = "net0"

				return c
			},

			expectedError: "name must be a valid IP address: ParseAddr(\"net4\"): unable to parse IP",
		},
		{
			name: "valid",
			cfg: func() *network.Layer2VIPConfigV1Alpha1 {
				c := network.NewLayer2VIPConfigV1Alpha1("fd00::1")
				c.LinkName = "net45"

				return c
			},
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
