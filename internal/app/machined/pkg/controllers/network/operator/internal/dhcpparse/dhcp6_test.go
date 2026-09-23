// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dhcpparse_test

import (
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/operator/internal/dhcpparse"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestParseDHCP6FQDN(t *testing.T) {
	// roundtrip the packet through the wire format, as a DHCPv6 server would send it
	wire := func(t *testing.T, modifiers ...dhcpv6.Modifier) *dhcpv6.Message {
		t.Helper()

		msg := must.Value(dhcpv6.NewMessage(modifiers...))(t)

		return must.Value(dhcpv6.MessageFromBytes(msg.ToBytes()))(t)
	}

	t.Run("no FQDN", func(t *testing.T) {
		assert.Nil(t, dhcpparse.ParseDHCP6FQDN(wire(t).Options.FQDN()))
	})

	t.Run("valid FQDN", func(t *testing.T) {
		specs := dhcpparse.ParseDHCP6FQDN(wire(t, dhcpv6.WithFQDN(0, "node1.example.com")).Options.FQDN())

		require.Len(t, specs, 1)
		assert.Equal(t, "node1.example.com", specs[0].Hostname)
		assert.Equal(t, network.ConfigOperator, specs[0].ConfigLayer)
	})

	t.Run("FQDN with newline", func(t *testing.T) {
		assert.Nil(t, dhcpparse.ParseDHCP6FQDN(wire(t, dhcpv6.WithFQDN(0, "node1.example.com\nregistry.k8s.io")).Options.FQDN()))
	})
}
