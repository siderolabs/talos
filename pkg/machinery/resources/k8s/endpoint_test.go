// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

func TestEndpointList(t *testing.T) {
	t.Parallel()

	var l k8s.EndpointList

	e1 := k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, "1")
	e1.TypedSpec().Addresses = []netip.Addr{
		netip.MustParseAddr("172.20.0.2"),
		netip.MustParseAddr("172.20.0.3"),
	}

	e2 := k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, "2")
	e2.TypedSpec().Addresses = []netip.Addr{
		netip.MustParseAddr("172.20.0.4"),
		netip.MustParseAddr("172.20.0.3"),
	}

	l = l.Merge(e1)
	l = l.Merge(e2)

	assert.Equal(t, []string{"172.20.0.2", "172.20.0.3", "172.20.0.4"}, l.Strings())
}

func TestEndpointListWithHosts(t *testing.T) {
	t.Parallel()

	var l k8s.EndpointList

	assert.True(t, l.IsEmpty())

	e1 := k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, "1")
	e1.TypedSpec().Addresses = []netip.Addr{
		netip.MustParseAddr("172.20.0.2"),
	}
	e1.TypedSpec().Hosts = []string{
		"host1.example.com",
		"host2.example.com",
	}

	e2 := k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, "2")
	e2.TypedSpec().Addresses = []netip.Addr{
		netip.MustParseAddr("172.20.0.3"),
	}
	e2.TypedSpec().Hosts = []string{
		"host2.example.com",
		"host3.example.com",
	}

	l = l.Merge(e1)
	l = l.Merge(e2)

	assert.Equal(t,
		[]string{
			"172.20.0.2",
			"172.20.0.3",
			"host1.example.com",
			"host2.example.com",
			"host3.example.com",
		},
		l.Strings(),
	)
}
