// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package addressutil_test

import (
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/addressutil"
)

func TestBroadcastAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prefix  string
		want    string
		wantNil bool
	}{
		{
			name:   "IPv4 /24 network",
			prefix: "10.255.255.231/24",
			want:   "10.255.255.255",
		},
		{
			name:   "IPv4 /16 network",
			prefix: "192.168.0.1/16",
			want:   "192.168.255.255",
		},
		{
			name:    "IPv4 /32 host route (VIP case)",
			prefix:  "10.255.255.230/32",
			wantNil: true, // Should return nil, not set broadcast
		},
		{
			name:    "Another /32 host route",
			prefix:  "192.168.1.100/32",
			wantNil: true, // Should return nil, not set broadcast
		},
		{
			name:    "IPv4 /31 point-to-point",
			prefix:  "10.0.0.1/31",
			wantNil: true, // RFC 3021 - /31 is point-to-point, no broadcast
		},
		{
			name:   "IPv4 /8 network",
			prefix: "10.0.0.1/8",
			want:   "10.255.255.255",
		},
		{
			name:    "IPv6 address (no broadcast)",
			prefix:  "2001:db8::1/64",
			wantNil: true, // IPv6 doesn't have broadcast
		},
		{
			name:    "IPv6 /128 address",
			prefix:  "2001:db8::1/128",
			wantNil: true, // IPv6 doesn't have broadcast
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefix, err := netip.ParsePrefix(tt.prefix)
			require.NoError(t, err)

			got := addressutil.BroadcastAddr(prefix)

			if tt.wantNil {
				assert.Nil(t, got, "expected nil broadcast for %s", tt.prefix)
			} else {
				assert.NotNil(t, got, "expected broadcast address for %s", tt.prefix)

				want := net.ParseIP(tt.want)
				assert.True(t, got.Equal(want), "expected %v, got %v for %s", want, got, tt.prefix)
			}
		})
	}
}
