// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

func TestResolverSpecMarshalYAML(t *testing.T) {
	spec := network.ResolverSpecSpec{
		DNSServers:  []netaddr.IP{netaddr.MustParseIP("1.1.1.1"), netaddr.MustParseIP("8.8.8.8")},
		ConfigLayer: network.ConfigPlatform,
	}

	marshaled, err := yaml.Marshal(spec)
	require.NoError(t, err)

	assert.Equal(t, "dnsServers:\n    - 1.1.1.1\n    - 8.8.8.8\nlayer: platform\n", string(marshaled))

	var spec2 network.ResolverSpecSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &spec2))

	assert.Equal(t, spec, spec2)
}
