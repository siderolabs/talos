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

//go:embed testdata/hcloudvipconfig.yaml
var expectedHCloudVIPConfigDocument []byte

func TestHCloudVIPConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewHCloudVIPConfigV1Alpha1("1.2.3.4")
	cfg.LinkName = "net33"
	cfg.APIToken = "s3cr3t-t0k3n"

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedHCloudVIPConfigDocument, marshaled)
}

func TestHCloudVIPConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedHCloudVIPConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	c := &network.HCloudVIPConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.HCloudVIPKind,
		},
		MetaName: "1.2.3.4",
		LinkName: "net33",
		APIToken: "s3cr3t-t0k3n",
	}

	assert.Equal(t, c, docs[0])
}

func TestHCloudVIPValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.HCloudVIPConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.HCloudVIPConfigV1Alpha1 {
				return network.NewHCloudVIPConfigV1Alpha1("")
			},

			expectedError: "name must be specified\nlink must be specified\napiToken must be specified",
		},
		{
			name: "no link name and token",
			cfg: func() *network.HCloudVIPConfigV1Alpha1 {
				return network.NewHCloudVIPConfigV1Alpha1("1.1.1.1")
			},

			expectedError: "link must be specified\napiToken must be specified",
		},
		{
			name: "invalid IP",
			cfg: func() *network.HCloudVIPConfigV1Alpha1 {
				c := network.NewHCloudVIPConfigV1Alpha1("net32")
				c.LinkName = "net32"
				c.APIToken = "foo"

				return c
			},

			expectedError: "name must be a valid IP address: ParseAddr(\"net32\"): unable to parse IP",
		},
		{
			name: "valid",
			cfg: func() *network.HCloudVIPConfigV1Alpha1 {
				c := network.NewHCloudVIPConfigV1Alpha1("fd00::1")
				c.LinkName = "net33"
				c.APIToken = "my-secret-token"

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
