// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

func TestPortRange(t *testing.T) {
	t.Run("MarshalYAML", func(t *testing.T) {
		for _, test := range []struct {
			name string
			pr   network.PortRange

			expected string
		}{
			{
				name: "single port",
				pr:   network.PortRange{Lo: 80, Hi: 80},

				expected: "80\n",
			},
			{
				name: "port range",
				pr:   network.PortRange{Lo: 80, Hi: 443},

				expected: "80-443\n",
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				marshaled, err := yaml.Marshal(test.pr)
				require.NoError(t, err)

				assert.Equal(t, test.expected, string(marshaled))
			})
		}
	})

	t.Run("UnmarshalYAML", func(t *testing.T) {
		for _, test := range []struct {
			name string
			yaml string

			expected network.PortRange
		}{
			{
				name: "single port",
				yaml: "80\n",

				expected: network.PortRange{Lo: 80, Hi: 80},
			},
			{
				name: "port range",
				yaml: "80-443\n",

				expected: network.PortRange{Lo: 80, Hi: 443},
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				var pr network.PortRange

				err := yaml.Unmarshal([]byte(test.yaml), &pr)
				require.NoError(t, err)

				assert.Equal(t, test.expected, pr)
			})
		}
	})
}

func TestPortRanges(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		for _, test := range []struct {
			name string
			prs  network.PortRanges

			expectedError string
		}{
			{
				name: "empty",
				prs:  network.PortRanges{},
			},
			{
				name: "valid",
				prs:  network.PortRanges{{Lo: 80, Hi: 80}, {Lo: 443, Hi: 443}, {Lo: 8080, Hi: 8081}},
			},
			{
				name: "inversion",
				prs:  network.PortRanges{{Lo: 8081, Hi: 8080}},

				expectedError: "invalid port range: 8081-8080",
			},
			{
				name: "overlap",
				prs:  network.PortRanges{{Lo: 1000, Hi: 2000}, {Lo: 80, Hi: 80}, {Lo: 1500, Hi: 2500}},

				expectedError: "invalid port range: 1500-2500, overlaps with 1000-2000",
			},
			{
				name: "duplicate",
				prs:  network.PortRanges{{Lo: 1000, Hi: 1000}, {Lo: 80, Hi: 80}, {Lo: 1000, Hi: 1000}},

				expectedError: "invalid port range: 1000-1000, overlaps with 1000-1000",
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				err := test.prs.Validate()
				if test.expectedError != "" {
					require.EqualError(t, err, test.expectedError)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}
