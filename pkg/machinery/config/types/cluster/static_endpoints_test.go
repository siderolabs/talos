// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	_ "embed"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

//go:embed testdata/staticendpointsconfig.yaml
var expectedStaticEndpointsConfigDocument []byte

func TestStaticEndpointsConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := cluster.NewStaticEndpointsConfigV1Alpha1()
	cfg.EndpointAddresses = []netip.Addr{netip.MustParseAddr("10.0.0.11"), netip.MustParseAddr("2001:db8::12")}
	cfg.EndpointHostnames = []string{"cp1.example.com"}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedStaticEndpointsConfigDocument, marshaled)
}

func TestStaticEndpointsConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedStaticEndpointsConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &cluster.StaticEndpointsConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       cluster.StaticEndpointsKind,
		},
		EndpointAddresses: []netip.Addr{netip.MustParseAddr("10.0.0.11"), netip.MustParseAddr("2001:db8::12")},
		EndpointHostnames: []string{"cp1.example.com"},
	}, docs[0])

	staticEndpoints := provider.StaticEndpointsConfig()
	require.NotNil(t, staticEndpoints)
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.11"), netip.MustParseAddr("2001:db8::12")}, staticEndpoints.Addresses())
	assert.Equal(t, []string{"cp1.example.com"}, staticEndpoints.Hostnames())
}

func TestStaticEndpointsConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *cluster.StaticEndpointsConfigV1Alpha1

		expectedError string
	}{
		{
			name: "valid",
			cfg: func() *cluster.StaticEndpointsConfigV1Alpha1 {
				cfg := cluster.NewStaticEndpointsConfigV1Alpha1()
				cfg.EndpointAddresses = []netip.Addr{netip.MustParseAddr("10.0.0.11"), netip.MustParseAddr("2001:db8::12")}
				cfg.EndpointHostnames = []string{"cp1.example.com", "cp2"}

				return cfg
			},
		},
		{
			name: "only hostnames",
			cfg: func() *cluster.StaticEndpointsConfigV1Alpha1 {
				cfg := cluster.NewStaticEndpointsConfigV1Alpha1()
				cfg.EndpointHostnames = []string{"cp1.example.com"}

				return cfg
			},
		},
		{
			name:          "empty",
			cfg:           cluster.NewStaticEndpointsConfigV1Alpha1,
			expectedError: "at least one address or hostname is required",
		},
		{
			name: "invalid addresses",
			cfg: func() *cluster.StaticEndpointsConfigV1Alpha1 {
				cfg := cluster.NewStaticEndpointsConfigV1Alpha1()
				cfg.EndpointAddresses = []netip.Addr{{}, netip.MustParseAddr("0.0.0.0")}

				return cfg
			},
			expectedError: "invalid address \"invalid IP\"\ninvalid address \"0.0.0.0\"",
		},
		{
			name: "invalid hostnames",
			cfg: func() *cluster.StaticEndpointsConfigV1Alpha1 {
				cfg := cluster.NewStaticEndpointsConfigV1Alpha1()
				cfg.EndpointHostnames = []string{"", "10.0.0.1", "cp1.example.com:6443", "https://cp1.example.com", "cp 1"}

				return cfg
			},
			expectedError: "hostname can't be empty\n" +
				"hostname \"10.0.0.1\" is an IP address, it should be listed in addresses\n" +
				"hostname \"cp1.example.com:6443\" should not include a port or a scheme\n" +
				"hostname \"https://cp1.example.com\" should not include a port or a scheme\n" +
				"name \"cp 1\" contains invalid character ' ' at position 2",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})
			assert.Nil(t, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
