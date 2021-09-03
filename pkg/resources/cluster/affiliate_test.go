// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/resources/cluster"
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
				Addresses:   []netaddr.IP{netaddr.MustParseIP("10.0.0.2")},
			},
			b: cluster.AffiliateSpec{
				Hostname:    "foo.com",
				Nodename:    "bar",
				MachineType: machine.TypeControlPlane,
				Addresses:   []netaddr.IP{netaddr.MustParseIP("10.0.0.2"), netaddr.MustParseIP("192.168.3.4")},
				KubeSpan: cluster.KubeSpanAffiliateSpec{
					PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
					Address:             netaddr.MustParseIP("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
					AdditionalAddresses: []netaddr.IPPrefix{netaddr.MustParseIPPrefix("10.244.3.1/24")},
					Endpoints:           []netaddr.IPPort{netaddr.MustParseIPPort("10.0.0.2:51820"), netaddr.MustParseIPPort("192.168.3.4:51820")},
				},
			},
			expected: cluster.AffiliateSpec{
				Hostname:    "foo.com",
				Nodename:    "bar",
				MachineType: machine.TypeControlPlane,
				Addresses:   []netaddr.IP{netaddr.MustParseIP("10.0.0.2"), netaddr.MustParseIP("192.168.3.4")},
				KubeSpan: cluster.KubeSpanAffiliateSpec{
					PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
					Address:             netaddr.MustParseIP("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
					AdditionalAddresses: []netaddr.IPPrefix{netaddr.MustParseIPPrefix("10.244.3.1/24")},
					Endpoints:           []netaddr.IPPort{netaddr.MustParseIPPort("10.0.0.2:51820"), netaddr.MustParseIPPort("192.168.3.4:51820")},
				},
			},
		},
		{
			name: "merge mixed",
			a: cluster.AffiliateSpec{
				Addresses: []netaddr.IP{netaddr.MustParseIP("10.0.0.2")},
				KubeSpan: cluster.KubeSpanAffiliateSpec{
					PublicKey: "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
					Address:   netaddr.MustParseIP("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
					Endpoints: []netaddr.IPPort{netaddr.MustParseIPPort("192.168.3.4:51820")},
				},
			},
			b: cluster.AffiliateSpec{
				Hostname:    "foo.com",
				Nodename:    "bar",
				MachineType: machine.TypeControlPlane,
				Addresses:   []netaddr.IP{netaddr.MustParseIP("192.168.3.4")},
				KubeSpan: cluster.KubeSpanAffiliateSpec{
					PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
					Address:             netaddr.MustParseIP("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
					AdditionalAddresses: []netaddr.IPPrefix{netaddr.MustParseIPPrefix("10.244.3.1/24")},
					Endpoints:           []netaddr.IPPort{netaddr.MustParseIPPort("10.0.0.2:51820"), netaddr.MustParseIPPort("192.168.3.4:51820")},
				},
			},
			expected: cluster.AffiliateSpec{
				Hostname:    "foo.com",
				Nodename:    "bar",
				MachineType: machine.TypeControlPlane,
				Addresses:   []netaddr.IP{netaddr.MustParseIP("10.0.0.2"), netaddr.MustParseIP("192.168.3.4")},
				KubeSpan: cluster.KubeSpanAffiliateSpec{
					PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
					Address:             netaddr.MustParseIP("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
					AdditionalAddresses: []netaddr.IPPrefix{netaddr.MustParseIPPrefix("10.244.3.1/24")},
					Endpoints:           []netaddr.IPPort{netaddr.MustParseIPPort("192.168.3.4:51820"), netaddr.MustParseIPPort("10.0.0.2:51820")},
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
