// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lldp_test

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/lldp"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func tlv(kind uint16, value ...byte) []byte {
	return append(binary.BigEndian.AppendUint16(nil, kind<<9|uint16(len(value))), value...)
}

func frame(values ...[]byte) []byte {
	result := []byte{1, 0x80, 0xc2, 0, 0, 0x0e, 2, 0, 0, 0, 0, 1, 0x88, 0xcc}
	for _, value := range values {
		result = append(result, value...)
	}

	return result
}

func advertisement(ttl uint16, optional ...[]byte) []byte {
	values := make([][]byte, 0, 3+len(optional)+1)
	values = append(values, tlv(1, 4, 2, 0, 0, 0, 0, 1), tlv(2, 5, 'e', 't', 'h', '0'), tlv(3, byte(ttl>>8), byte(ttl)))
	values = append(values, optional...)
	values = append(values, tlv(0))

	return frame(values...)
}

func management(ip string) []byte {
	address := netip.MustParseAddr(ip)
	family := byte(2)

	if address.Is4() {
		family = 1
	}

	value := append([]byte{byte(address.BitLen()/8 + 1), family}, address.AsSlice()...)
	value = append(value, 2, 0, 0, 0, 1, 0) // interface subtype, number and empty OID

	return tlv(8, value...)
}

func pvid(id uint16) []byte {
	return tlv(127, 0, 0x80, 0xc2, 1, byte(id>>8), byte(id))
}

func vlanName(id uint16, name string) []byte {
	value := make([]byte, 0, 7+len(name))
	value = append(value, 0, 0x80, 0xc2, 3, byte(id>>8), byte(id), byte(len(name)))

	return tlv(127, append(value, name...)...)
}

func TestDecodeFrame(t *testing.T) {
	t.Parallel()

	neighbor, err := lldp.DecodeFrame(advertisement(120,
		tlv(4, []byte("uplink")...), tlv(5, []byte("switch")...), tlv(6, []byte("switch OS")...),
		management("2001:db8::1"), management("192.0.2.1"), management("192.0.2.1"),
		vlanName(20, "servers"), pvid(10), vlanName(10, "clients"), pvid(20),
	))
	require.NoError(t, err)
	assert.Equal(t, uint16(120), neighbor.TTL)
	assert.NotEmpty(t, neighbor.Key)
	assert.Equal(t, network.LLDPNeighborSpec{
		ChassisID: "02:00:00:00:00:01", PortID: "eth0", PortDescription: "uplink",
		SystemName: "switch", SystemDescription: "switch OS",
		ManagementAddresses: []string{"192.0.2.1", "2001:db8::1"},
		VLANs:               []network.LLDPVLANSpec{{ID: 10, Name: "clients"}, {ID: 20, Name: "servers"}},
	}, neighbor.Spec)
}

func TestWithdrawalAndRawIdentity(t *testing.T) {
	t.Parallel()

	live, err := lldp.DecodeFrame(advertisement(120))
	require.NoError(t, err)
	withdrawal, err := lldp.DecodeFrame(advertisement(0))
	require.NoError(t, err)
	assert.Zero(t, withdrawal.TTL)
	assert.Equal(t, live.Key, withdrawal.Key)

	// A textual MAC and a MAC subtype render alike, but are different neighbors.
	text, err := lldp.DecodeFrame(frame(tlv(1, append([]byte{7}, []byte(live.Spec.ChassisID)...)...),
		tlv(2, 5, 'e', 't', 'h', '0'), tlv(3, 0, 120), tlv(0)))
	require.NoError(t, err)
	assert.Equal(t, live.Spec, text.Spec)
	assert.NotEqual(t, live.Key, text.Key)

	localPort, err := lldp.DecodeFrame(frame(tlv(1, 4, 2, 0, 0, 0, 0, 1),
		tlv(2, 7, 'e', 't', 'h', '0'), tlv(3, 0, 120), tlv(0)))
	require.NoError(t, err)
	assert.Equal(t, live.Spec, localPort.Spec)
	assert.NotEqual(t, live.Key, localPort.Key)

	// Keys and specs own their bytes even when a listener reuses the frame buffer.
	buffer := advertisement(120)
	owned, err := lldp.DecodeFrame(buffer)
	require.NoError(t, err)
	clear(buffer)
	assert.Equal(t, live, owned)
}

func TestIDFormatting(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		id   []byte
		want string
	}{
		{"text", []byte{7, 's', 'w'}, "sw"},
		{"ipv4", []byte{5, 1, 192, 0, 2, 1}, "192.0.2.1"},
		{"ipv6", append([]byte{5, 2}, netip.MustParseAddr("2001:db8::1").AsSlice()...), "2001:db8::1"},
		// Unrenderable identifiers are dropped: the neighbor is still tracked by its raw key.
		{"unknown address family", []byte{5, 42, 1, 2}, ""},
		{"control", []byte{7, 'a', 0, 'b'}, ""},
		{"invalid utf8", []byte{7, 0xff}, ""},
		{"text is verbatim", []byte{7, 'h', 'e', 'x', ':', 'f', 'f'}, "hex:ff"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			neighbor, err := lldp.DecodeFrame(frame(tlv(1, test.id...), tlv(2, 7, 'p'), tlv(3, 0, 1), tlv(0)))
			require.NoError(t, err)
			assert.Equal(t, test.want, neighbor.Spec.ChassisID)
		})
	}
}

func TestIgnoredTLVs(t *testing.T) {
	t.Parallel()

	base, err := lldp.DecodeFrame(advertisement(120))
	require.NoError(t, err)

	for _, value := range [][]byte{
		tlv(7), // Unsupported system-capability semantics.
		tlv(9, 0xff),
		tlv(127, 0, 0x12, 0x0f, 1), // Truncated 802.3 MAC/PHY payload: ignored.
		tlv(127, 0, 0x12, 0xbb, 1), // Truncated LLDP-MED payload: ignored.
		tlv(127, 0, 0x80, 0xc2, 2), // Unsupported 802.1 port/protocol VLAN.
		tlv(127, 0, 0x80, 0xc2, 7), // Unsupported 802.1 aggregation.
		tlv(127, 0xaa, 0xbb, 0xcc, 255, 0xff),
	} {
		neighbor, decodeErr := lldp.DecodeFrame(advertisement(120, value))
		require.NoError(t, decodeErr)
		assert.Equal(t, base, neighbor)
	}
}

func TestVLANMerging(t *testing.T) {
	t.Parallel()

	// A name always wins over a bare PVID, whichever comes first. Two names for
	// one VLAN are a switch misconfiguration, and the first one wins.
	for name, values := range map[string][][]byte{
		"names first": {vlanName(20, "z"), vlanName(20, "a"), pvid(20), pvid(10)},
		"pvids first": {pvid(20), pvid(10), vlanName(20, "z"), vlanName(20, "a")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			neighbor, err := lldp.DecodeFrame(advertisement(1, values...))
			require.NoError(t, err)
			assert.Equal(t, []network.LLDPVLANSpec{{ID: 10}, {ID: 20, Name: "z"}}, neighbor.Spec.VLANs)
		})
	}
}

func TestUnrenderableFieldsAreDropped(t *testing.T) {
	t.Parallel()

	base, err := lldp.DecodeFrame(advertisement(120))
	require.NoError(t, err)

	neighbor, err := lldp.DecodeFrame(advertisement(120,
		tlv(4, 'u', 0x07, 'p'),                // port description carrying a control character
		tlv(5, 0xff, 0xfe),                    // system name which is not valid UTF-8
		tlv(6, 's', '\n', 'w'),                // multi-line system description
		tlv(8, 3, 42, 1, 2, 2, 0, 0, 0, 1, 0), // management address in an unknown family
		vlanName(10, "a\x00b"),                // VLAN name carrying a control character
	))
	require.NoError(t, err)
	assert.Empty(t, neighbor.Spec.PortDescription)
	assert.Empty(t, neighbor.Spec.SystemName)
	assert.Empty(t, neighbor.Spec.SystemDescription)
	assert.Empty(t, neighbor.Spec.ManagementAddresses)
	assert.Equal(t, []network.LLDPVLANSpec{{ID: 10}}, neighbor.Spec.VLANs)
	assert.Equal(t, base.Key, neighbor.Key)
}

func TestMalformedFrames(t *testing.T) {
	t.Parallel()

	chassis, port, ttl, end := tlv(1, 7, 'c'), tlv(2, 7, 'p'), tlv(3, 0, 1), tlv(0)

	for name, value := range map[string][]byte{
		"empty":                     nil,
		"short ethernet":            {1, 2},
		"missing mandatory":         frame(end),
		"wrong order":               frame(port, chassis, ttl, end),
		"duplicate mandatory":       frame(chassis, port, ttl, chassis, end),
		"missing end":               frame(chassis, port, ttl),
		"partial header":            frame(chassis, port, ttl, []byte{0}),
		"truncated value":           frame(chassis, port, ttl, []byte{0xfe, 0x05, 1}),
		"nonempty end":              frame(chassis, port, ttl, tlv(0, 1)),
		"short chassis":             frame(tlv(1, 7), port, ttl, end),
		"invalid subtype":           frame(tlv(1, 0, 'c'), port, ttl, end),
		"invalid port subtype":      frame(chassis, tlv(2, 8, 'p'), ttl, end),
		"short mac":                 frame(tlv(1, 4, 1), port, ttl, end),
		"short ip":                  frame(tlv(1, 5, 1, 192), port, ttl, end),
		"short ttl":                 frame(chassis, port, tlv(3, 0), end),
		"short management":          advertisement(1, tlv(8)),
		"short management oid":      advertisement(1, tlv(8, 5, 1, 192, 0, 2, 1, 2, 0, 0, 0, 1, 1)),
		"short organization header": advertisement(1, tlv(127, 0, 0x80, 0xc2)),
		"short pvid":                advertisement(1, tlv(127, 0, 0x80, 0xc2, 1)),
		"short vlan name":           advertisement(1, tlv(127, 0, 0x80, 0xc2, 3, 0, 1, 3, 'a')),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := lldp.DecodeFrame(value)
			require.Error(t, err)
		})
	}
}

func TestEveryTruncatedPrefix(t *testing.T) {
	t.Parallel()

	complete := advertisement(120, management("2001:db8::1"), vlanName(10, "clients"))

	for length := range complete {
		_, err := lldp.DecodeFrame(complete[:length])
		require.Error(t, err, "prefix length %d", length)
	}

	_, err := lldp.DecodeFrame(complete)
	require.NoError(t, err)
}

func TestEthernetEnvelope(t *testing.T) {
	t.Parallel()

	plain := advertisement(1)
	want, err := lldp.DecodeFrame(plain)
	require.NoError(t, err)

	for name, padding := range map[string][]byte{
		"none":                  nil,
		"single zero":           {0},
		"zero padding":          make([]byte, 46),
		"nonzero padding":       {0xff, 0xff, 0xff},
		"partial TLV header":    {0xfe},
		"truncated TLV value":   {0xfe, 0x05, 1},
		"mandatory TLV padding": tlv(1, 7, 'x'),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			padded, decodeErr := lldp.DecodeFrame(append(advertisement(1), padding...))
			require.NoError(t, decodeErr)
			assert.Equal(t, want, padded)
		})
	}

	plain[12], plain[13] = 8, 0
	_, err = lldp.DecodeFrame(plain)
	require.Error(t, err)
}
