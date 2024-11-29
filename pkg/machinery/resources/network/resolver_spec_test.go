// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestResolverSpecMarshalYAML(t *testing.T) {
	spec := network.ResolverSpecSpec{
		DNSServers:    []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("8.8.8.8")},
		ConfigLayer:   network.ConfigPlatform,
		SearchDomains: []string{"example.com"},
	}

	marshaled, err := yaml.Marshal(spec)
	require.NoError(t, err)

	assert.Equal(t, "dnsServers:\n    - 1.1.1.1\n    - 8.8.8.8\nlayer: platform\nsearchDomains:\n    - example.com\n", string(marshaled))

	var spec2 network.ResolverSpecSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &spec2))

	assert.Equal(t, spec, spec2)
}
