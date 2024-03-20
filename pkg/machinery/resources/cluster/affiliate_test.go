// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"net/netip"
	"testing"

	"github.com/siderolabs/protoenc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cluster2 "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/proto"
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
				Hostname:     "foo.com",
				Nodename:     "bar",
				MachineType:  machine.TypeControlPlane,
				Addresses:    []netip.Addr{netip.MustParseAddr("10.0.0.2")},
				ControlPlane: &cluster.ControlPlane{APIServerPort: 6443},
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
				ControlPlane: &cluster.ControlPlane{APIServerPort: 6443},
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
				ControlPlane: &cluster.ControlPlane{APIServerPort: 6443},
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
				ControlPlane: &cluster.ControlPlane{APIServerPort: 6443},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spec := tt.a
			spec.Merge(&tt.b)

			assert.Equal(t, tt.expected, spec)
		})
	}
}

func TestAffiliateSpecMarshal(t *testing.T) {
	original := &cluster.AffiliateSpec{
		NodeID:       "myNodeID",
		Hostname:     "foo.com",
		MachineType:  machine.TypeControlPlane,
		ControlPlane: &cluster.ControlPlane{APIServerPort: 6443},
	}

	wire, err := protoenc.Marshal(original)
	require.NoError(t, err)

	var unmarshaled cluster2.AffiliateSpec

	err = proto.Unmarshal(wire, &unmarshaled)
	require.NoError(t, err)

	require.EqualValues(t, original.NodeID, unmarshaled.NodeId)
	require.EqualValues(t, original.Hostname, unmarshaled.Hostname)
	require.EqualValues(t, original.MachineType, unmarshaled.MachineType)
	require.EqualValues(t, original.ControlPlane.APIServerPort, unmarshaled.ControlPlane.ApiServerPort)

	unmarshaled.ControlPlane = nil

	wire, err = proto.Marshal(&unmarshaled)
	require.NoError(t, err)

	spec := &cluster.AffiliateSpec{}
	err = protoenc.Unmarshal(wire, spec)
	require.NoError(t, err)

	original.ControlPlane = nil

	require.Equal(t, original, spec)
}
