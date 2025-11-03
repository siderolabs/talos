// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestAddressSpecMarshalYAML(t *testing.T) {
	spec := network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("192.168.3.6/27"),
		LinkName:    "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeLink,
		Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	marshaled, err := yaml.Marshal(spec)
	require.NoError(t, err)

	assert.Equal(t, "address: 192.168.3.6/27\nlinkName: eth0\nfamily: inet4\nscope: link\nflags: permanent\nlayer: configuration\n", string(marshaled))

	var spec2 network.AddressSpecSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &spec2))

	assert.Equal(t, spec, spec2)
}
