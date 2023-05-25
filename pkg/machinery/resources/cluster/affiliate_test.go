// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

func TestAffiliateSpec_Merge(t *testing.T) {
	for _, tt := range []struct {
		name           string
		a, b, expected cluster.AffiliateSpec
	}{
		{
			name: "zero",
		},
		{
			name: "merge kubespan",
			a: cluster.AffiliateSpec{
				Hostname:    "foo.com",
				Nodename:    "bar",
				MachineType: machine.TypeControlPlane,
				Addresses:   []netip.Addr{netip.MustParseAddr("10.0.0.2")},
			},
			b: cluster.AffiliateSpec{
				Hostname:    "foo.com",
				Nodename:    "bar",
				MachineType: machine.TypeControlPlane,
				Addresses:   []netip.Addr{netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("192.168.3.4")},
				KubeSpan: cluster.KubeSpanAffiliateSpec{
					PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
					Address:             netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
					AdditionalAddresses: []netip.Prefix{netip.MustParsePrefix("10.244.3.1/24")},
					Endpoints:           []netip.AddrPort{netip.MustParseAddrPort("10.0.0.2:51820"), netip.MustParseAddrPort("192.168.3.4:51820")},
				},
			},
			expected: cluster.AffiliateSpec{
				Hostname:    "foo.com",
				Nodename:    "bar",
				MachineType: machine.TypeControlPlane,
				Addresses:   []netip.Addr{netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("192.168.3.4")},
				KubeSpan: cluster.KubeSpanAffiliateSpec{
					PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
					Address:             netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
					AdditionalAddresses: []netip.Prefix{netip.MustParsePrefix("10.244.3.1/24")},
					Endpoints:           []netip.AddrPort{netip.MustParseAddrPort("10.0.0.2:51820"), netip.MustParseAddrPort("192.168.3.4:51820")},
				},
			},
		},
		{
			name: "merge mixed",
			a: cluster.AffiliateSpec{
				Addresses: []netip.Addr{netip.MustParseAddr("10.0.0.2")},
				KubeSpan: cluster.KubeSpanAffiliateSpec{
					PublicKey: "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
					Address:   netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
					Endpoints: []netip.AddrPort{netip.MustParseAddrPort("192.168.3.4:51820")},
				},
			},
			b: cluster.AffiliateSpec{
				Hostname:    "foo.com",
				Nodename:    "bar",
				MachineType: machine.TypeControlPlane,
				Addresses:   []netip.Addr{netip.MustParseAddr("192.168.3.4")},
				KubeSpan: cluster.KubeSpanAffiliateSpec{
					PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
					Address:             netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
					AdditionalAddresses: []netip.Prefix{netip.MustParsePrefix("10.244.3.1/24")},
					Endpoints:           []netip.AddrPort{netip.MustParseAddrPort("10.0.0.2:51820"), netip.MustParseAddrPort("192.168.3.4:51820")},
				},
			},
			expected: cluster.AffiliateSpec{
				Hostname:    "foo.com",
				Nodename:    "bar",
				MachineType: machine.TypeControlPlane,
				Addresses:   []netip.Addr{netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("192.168.3.4")},
				KubeSpan: cluster.KubeSpanAffiliateSpec{
					PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
					Address:             netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
					AdditionalAddresses: []netip.Prefix{netip.MustParsePrefix("10.244.3.1/24")},
					Endpoints:           []netip.AddrPort{netip.MustParseAddrPort("192.168.3.4:51820"), netip.MustParseAddrPort("10.0.0.2:51820")},
				},
			},
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			spec := tt.a
			spec.Merge(&tt.b)

			assert.Equal(t, tt.expected, spec)
		})
	}
}
