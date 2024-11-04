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

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestRoutSpecMarshalYAML(t *testing.T) {
	spec := network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet6,
		Destination: netip.MustParsePrefix("192.168.3.4/25"),
		Source:      netip.MustParseAddr("1.1.1.1"),
		Gateway:     netip.MustParseAddr("2.2.2.2"),
		OutLinkName: "eth0",
		Table:       nethelpers.TableLocal,
		Priority:    1024,
		Scope:       nethelpers.ScopeHost,
		Type:        nethelpers.TypeAnycast,
		Flags:       nethelpers.RouteFlags(nethelpers.RouteOffload | nethelpers.RouteCloned),
		Protocol:    nethelpers.ProtocolBoot,
		ConfigLayer: network.ConfigPlatform,
		MTU:         1400,
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
mtu: 1400
`,
		string(marshaled))

	var spec2 network.RouteSpecSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &spec2))

	assert.Equal(t, spec, spec2)
}

func TestRoutSpecNormalize(t *testing.T) {
	spec := network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: netip.MustParsePrefix("0.0.0.0/0"),
		Source:      netip.MustParseAddr("0.0.0.0"),
		Gateway:     netip.MustParseAddr("0.0.0.0"),
		OutLinkName: "eth0",
		Table:       nethelpers.TableLocal,
		Priority:    1024,
		ConfigLayer: network.ConfigPlatform,
		MTU:         1400,
	}

	normalizedFamily := spec.Normalize()

	assert.Equal(t, netip.Prefix{}, spec.Destination)
	assert.Equal(t, netip.Addr{}, spec.Source)
	assert.Equal(t, netip.Addr{}, spec.Gateway)
	assert.Equal(t, nethelpers.FamilyInet4, normalizedFamily)
	assert.Equal(t, nethelpers.ScopeGlobal, spec.Scope)
}

func TestRoutSpecNormalizeV6(t *testing.T) {
	spec := network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: netip.MustParsePrefix("::/0"),
		OutLinkName: "eth0",
		Table:       nethelpers.TableLocal,
		Priority:    1024,
		ConfigLayer: network.ConfigPlatform,
		MTU:         1400,
	}

	normalizedFamily := spec.Normalize()

	assert.Equal(t, netip.Prefix{}, spec.Destination)
	assert.Equal(t, netip.Addr{}, spec.Source)
	assert.Equal(t, netip.Addr{}, spec.Gateway)
	assert.Equal(t, nethelpers.FamilyInet6, normalizedFamily)
	assert.Equal(t, nethelpers.ScopeGlobal, spec.Scope)
}
