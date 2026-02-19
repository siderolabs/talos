// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeaccess_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/utils/ptr"

	kubeaccessctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/kubeaccess"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

func TestPopulateEndpointSlice(t *testing.T) {
	t.Parallel()

	//nolint:dupl
	for _, tt := range []struct {
		name             string
		existingSlice    *discoveryv1.EndpointSlice
		endpointAddrs    k8s.EndpointList
		expectedEndpoint []discoveryv1.Endpoint
	}{
		{
			name:          "empty endpoint slice, single address",
			existingSlice: &discoveryv1.EndpointSlice{},
			endpointAddrs: k8s.EndpointList{
				Addresses: []netip.Addr{
					netip.MustParseAddr("192.168.1.1"),
				},
			},
			expectedEndpoint: []discoveryv1.Endpoint{
				{
					Addresses: []string{"192.168.1.1"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
				},
			},
		},
		{
			name:          "empty endpoint slice, multiple addresses",
			existingSlice: &discoveryv1.EndpointSlice{},
			endpointAddrs: k8s.EndpointList{
				Addresses: []netip.Addr{
					netip.MustParseAddr("192.168.1.1"),
					netip.MustParseAddr("192.168.1.2"),
					netip.MustParseAddr("192.168.1.3"),
				},
			},
			expectedEndpoint: []discoveryv1.Endpoint{
				{
					Addresses: []string{"192.168.1.1"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
				},
				{
					Addresses: []string{"192.168.1.2"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
				},
				{
					Addresses: []string{"192.168.1.3"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
				},
			},
		},
		{
			name: "stale endpoints are removed",
			existingSlice: &discoveryv1.EndpointSlice{
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.0.0.1"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       ptr.To(true),
							Serving:     ptr.To(true),
							Terminating: ptr.To(false),
						},
					},
					{
						Addresses: []string{"10.0.0.2"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       ptr.To(true),
							Serving:     ptr.To(true),
							Terminating: ptr.To(false),
						},
					},
					{
						Addresses: []string{"10.0.0.3"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       ptr.To(true),
							Serving:     ptr.To(true),
							Terminating: ptr.To(false),
						},
					},
				},
			},
			endpointAddrs: k8s.EndpointList{
				Addresses: []netip.Addr{
					netip.MustParseAddr("10.0.0.1"),
				},
			},
			expectedEndpoint: []discoveryv1.Endpoint{
				{
					Addresses: []string{"10.0.0.1"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
				},
			},
		},
		{
			name: "all stale endpoints replaced with new ones",
			existingSlice: &discoveryv1.EndpointSlice{
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.0.0.1"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       ptr.To(true),
							Serving:     ptr.To(true),
							Terminating: ptr.To(false),
						},
					},
					{
						Addresses: []string{"10.0.0.2"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       ptr.To(true),
							Serving:     ptr.To(true),
							Terminating: ptr.To(false),
						},
					},
				},
			},
			endpointAddrs: k8s.EndpointList{
				Addresses: []netip.Addr{
					netip.MustParseAddr("192.168.1.1"),
					netip.MustParseAddr("192.168.1.2"),
				},
			},
			expectedEndpoint: []discoveryv1.Endpoint{
				{
					Addresses: []string{"192.168.1.1"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
				},
				{
					Addresses: []string{"192.168.1.2"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
				},
			},
		},
		{
			name: "duplicate addresses are deduplicated",
			existingSlice: &discoveryv1.EndpointSlice{
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.0.0.1"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       ptr.To(true),
							Serving:     ptr.To(true),
							Terminating: ptr.To(false),
						},
					},
				},
			},
			endpointAddrs: k8s.EndpointList{
				Addresses: []netip.Addr{
					netip.MustParseAddr("192.168.1.1"),
					netip.MustParseAddr("192.168.1.1"),
					netip.MustParseAddr("192.168.1.2"),
				},
			},
			expectedEndpoint: []discoveryv1.Endpoint{
				{
					Addresses: []string{"192.168.1.1"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
				},
				{
					Addresses: []string{"192.168.1.2"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
				},
			},
		},
		{
			name: "empty address list clears endpoints",
			existingSlice: &discoveryv1.EndpointSlice{
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.0.0.1"},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       ptr.To(true),
							Serving:     ptr.To(true),
							Terminating: ptr.To(false),
						},
					},
				},
			},
			endpointAddrs:    k8s.EndpointList{},
			expectedEndpoint: nil,
		},
		{
			name:          "IPv6 addresses",
			existingSlice: &discoveryv1.EndpointSlice{},
			endpointAddrs: k8s.EndpointList{
				Addresses: []netip.Addr{
					netip.MustParseAddr("fd00::1"),
					netip.MustParseAddr("fd00::2"),
				},
			},
			expectedEndpoint: []discoveryv1.Endpoint{
				{
					Addresses: []string{"fd00::1"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
				},
				{
					Addresses: []string{"fd00::2"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			kubeaccessctrl.PopulateEndpointSlice(tt.existingSlice, tt.endpointAddrs)

			assert.Equal(t, tt.expectedEndpoint, tt.existingSlice.Endpoints)

			// verify ports are always set correctly
			require.Len(t, tt.existingSlice.Ports, 1)
			assert.Equal(t, "apid", *tt.existingSlice.Ports[0].Name)
			assert.Equal(t, int32(constants.ApidPort), *tt.existingSlice.Ports[0].Port)
			assert.Equal(t, corev1.ProtocolTCP, *tt.existingSlice.Ports[0].Protocol)
		})
	}
}
