// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"inet.af/netaddr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/talos-systems/talos/internal/pkg/discovery/registry"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
)

func TestAnnotationsFromAffiliate(t *testing.T) {
	for _, tt := range []struct {
		name      string
		affiliate cluster.AffiliateSpec
		expected  map[string]string
	}{
		{
			name: "zero",
			expected: map[string]string{
				"cluster.talos.dev/node-id":                "",
				"networking.talos.dev/assigned-prefixes":   "",
				"networking.talos.dev/kubespan-endpoints":  "",
				"networking.talos.dev/kubespan-ip":         "",
				"networking.talos.dev/kubespan-public-key": "",
				"networking.talos.dev/self-ips":            "",
			},
		},
		{
			name: "mixed",
			affiliate: cluster.AffiliateSpec{
				NodeID:      "29QQTc97U5ZyFTIX33Dp9NqtwxqQI8QI13scCLzffrZ",
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
			expected: map[string]string{
				"cluster.talos.dev/node-id":                "29QQTc97U5ZyFTIX33Dp9NqtwxqQI8QI13scCLzffrZ",
				"networking.talos.dev/assigned-prefixes":   "10.244.3.1/24",
				"networking.talos.dev/kubespan-endpoints":  "10.0.0.2:51820,192.168.3.4:51820",
				"networking.talos.dev/kubespan-ip":         "fd50:8d60:4238:6302:f857:23ff:fe21:d1e0",
				"networking.talos.dev/kubespan-public-key": "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
				"networking.talos.dev/self-ips":            "10.0.0.2,192.168.3.4",
			},
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			affiliate := cluster.NewAffiliate(cluster.NamespaceName, tt.affiliate.NodeID)
			*affiliate.TypedSpec() = tt.affiliate

			assert.Equal(t, tt.expected, registry.AnnotationsFromAffiliate(affiliate))
		})
	}
}

func TestAffiliateFromNode(t *testing.T) {
	for _, tt := range []struct {
		name     string
		node     v1.Node
		expected *cluster.AffiliateSpec
	}{
		{
			name: "no annotations",
			node: v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "worker-1",
					Annotations: map[string]string{},
				},
				Spec: v1.NodeSpec{},
			},
			expected: nil,
		},
		{
			name: "discovered",
			node: v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bar",
					Annotations: map[string]string{
						"cluster.talos.dev/node-id":                "29QQTc97U5ZyFTIX33Dp9NqtwxqQI8QI13scCLzffrZ",
						"networking.talos.dev/assigned-prefixes":   "10.244.3.1/24",
						"networking.talos.dev/kubespan-endpoints":  "10.0.0.2:51820,192.168.3.4:51820",
						"networking.talos.dev/kubespan-ip":         "fd50:8d60:4238:6302:f857:23ff:fe21:d1e0",
						"networking.talos.dev/kubespan-public-key": "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
						"networking.talos.dev/self-ips":            "10.0.0.2,192.168.3.4",
					},
					Labels: map[string]string{
						constants.LabelNodeRoleControlPlane: "",
					},
				},
				Spec: v1.NodeSpec{},
				Status: v1.NodeStatus{
					Addresses: []v1.NodeAddress{
						{
							Type:    v1.NodeHostName,
							Address: "foo.com",
						},
					},
					NodeInfo: v1.NodeSystemInfo{
						OSImage: "Talos (v1.0.0)",
					},
				},
			},
			expected: &cluster.AffiliateSpec{
				NodeID:          "29QQTc97U5ZyFTIX33Dp9NqtwxqQI8QI13scCLzffrZ",
				Hostname:        "foo.com",
				Nodename:        "bar",
				MachineType:     machine.TypeControlPlane,
				Addresses:       []netaddr.IP{netaddr.MustParseIP("10.0.0.2"), netaddr.MustParseIP("192.168.3.4")},
				OperatingSystem: "Talos (v1.0.0)",
				KubeSpan: cluster.KubeSpanAffiliateSpec{
					PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
					Address:             netaddr.MustParseIP("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
					AdditionalAddresses: []netaddr.IPPrefix{netaddr.MustParseIPPrefix("10.244.3.1/24")},
					Endpoints:           []netaddr.IPPort{netaddr.MustParseIPPort("10.0.0.2:51820"), netaddr.MustParseIPPort("192.168.3.4:51820")},
				},
			},
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, registry.AffiliateFromNode(&tt.node))
		})
	}
}
