// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

func TestEndpointList(t *testing.T) {
	t.Parallel()

	var l k8s.EndpointList

	e1 := k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, "1")
	e1.TypedSpec().Addresses = []netaddr.IP{
		netaddr.MustParseIP("172.20.0.2"),
		netaddr.MustParseIP("172.20.0.3"),
	}

	e2 := k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, "2")
	e2.TypedSpec().Addresses = []netaddr.IP{
		netaddr.MustParseIP("172.20.0.4"),
		netaddr.MustParseIP("172.20.0.3"),
	}

	l = l.Merge(e1)
	l = l.Merge(e2)

	assert.Equal(t, []string{"172.20.0.2", "172.20.0.3", "172.20.0.4"}, l.Strings())
}
