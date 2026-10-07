// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain_test

import (
	"encoding/binary"
	"net/netip"
	"testing"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
)

func encodeGuestInterfaces(interfaces []libvirt.DomainInterface) []byte {
	payload := binary.BigEndian.AppendUint32(nil, uint32(len(interfaces)))

	for _, iface := range interfaces {
		payload = append(payload, encodeString(iface.Name)...)
		payload = binary.BigEndian.AppendUint32(payload, uint32(len(iface.Hwaddr)))

		for _, addr := range iface.Hwaddr {
			payload = append(payload, encodeString(addr)...)
		}

		payload = binary.BigEndian.AppendUint32(payload, uint32(len(iface.Addrs)))

		for _, addr := range iface.Addrs {
			payload = binary.BigEndian.AppendUint32(payload, uint32(addr.Type))
			payload = append(payload, encodeString(addr.Addr)...)
			payload = binary.BigEndian.AppendUint32(payload, addr.Prefix)
		}
	}

	return payload
}

func TestGuestInterfaces(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		wantError string
		addresses []libvirt.DomainIPAddr
		want      []netip.Prefix
	}{
		{
			name: "host addresses",
			addresses: []libvirt.DomainIPAddr{
				{Addr: "10.0.0.5", Prefix: 24},
				{Type: 1, Addr: "2001:db8::5", Prefix: 64},
			},
			want: []netip.Prefix{netip.MustParsePrefix("10.0.0.5/24"), netip.MustParsePrefix("2001:db8::5/64")},
		},
		{
			name:      "invalid address",
			addresses: []libvirt.DomainIPAddr{{Addr: "not-an-ip", Prefix: 24}},
			wantError: "parse guest interface",
		},
		{
			name:      "invalid IPv4 prefix",
			addresses: []libvirt.DomainIPAddr{{Addr: "10.0.0.5", Prefix: 33}},
			wantError: "invalid guest interface",
		},
		{
			name:      "invalid IPv6 prefix",
			addresses: []libvirt.DomainIPAddr{{Type: 1, Addr: "2001:db8::5", Prefix: 129}},
			wantError: "invalid guest interface",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			domain := libvirtdomain.Domain{Name: "guest", UUID: uuid.New()}
			client, _, served := openDomainFixture(t, domainRecord{
				identity: domain,
				interfaces: []libvirt.DomainInterface{
					{Name: "eth0", Hwaddr: libvirt.OptString{"52:54:00:12:34:56"}, Addrs: tt.addresses},
				},
			})
			interfaces, err := client.GuestInterfaces(domain)
			client.Close()
			require.NoError(t, <-served)

			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError, "malformed guest addresses must not enter domain status")
				require.Nil(t, interfaces)

				return
			}

			require.NoError(t, err)
			require.Equal(t, []libvirtdomain.GuestInterface{
				{Name: "eth0", HardwareAddr: "52:54:00:12:34:56", IPs: tt.want},
			}, interfaces, "guest addresses must retain host bits instead of becoming network addresses")
		})
	}
}
