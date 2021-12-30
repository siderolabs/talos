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

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

func TestRoutSpecMarshalYAML(t *testing.T) {
	spec := network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet6,
		Destination: netaddr.MustParseIPPrefix("192.168.3.4/25"),
		Source:      netaddr.MustParseIP("1.1.1.1"),
		Gateway:     netaddr.MustParseIP("2.2.2.2"),
		OutLinkName: "eth0",
		Table:       nethelpers.TableLocal,
		Priority:    1024,
		Scope:       nethelpers.ScopeHost,
		Type:        nethelpers.TypeAnycast,
		Flags:       nethelpers.RouteFlags(nethelpers.RouteOffload | nethelpers.RouteCloned),
		Protocol:    nethelpers.ProtocolBoot,
		ConfigLayer: network.ConfigPlatform,
	}

	marshaled, err := yaml.Marshal(spec)
	require.NoError(t, err)

	assert.Equal(t,
		`family: inet6
dst: 192.168.3.4/25
src: 1.1.1.1
gateway: 2.2.2.2
outLinkName: eth0
table: local
priority: 1024
scope: host
type: anycast
flags: cloned,offload
protocol: boot
layer: platform
`,
		string(marshaled))

	var spec2 network.RouteSpecSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &spec2))

	assert.Equal(t, spec, spec2)
}
