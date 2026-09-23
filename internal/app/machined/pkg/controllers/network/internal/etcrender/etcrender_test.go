// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcrender_test

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/etcrender"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestRenderResolvConfInvalidSearchDomains(t *testing.T) {
	t.Parallel()

	nameservers := []netip.Addr{netip.MustParseAddr("1.1.1.1")}

	assert.Equal(
		t,
		"nameserver 1.1.1.1\n\nsearch legit.example Corp_Example.com\n",
		string(etcrender.ResolvConf(slices.All(nameservers), []string{
			"legit.example",
			"poc.example\nnameserver 6.6.6.6",
			"foo bar",
			"Corp_Example.com",
		})),
	)

	assert.Equal(
		t,
		"nameserver 1.1.1.1\n",
		string(etcrender.ResolvConf(slices.All(nameservers), []string{"poc.example\nnameserver 6.6.6.6"})),
	)
}

func TestRenderHostsInvalidNames(t *testing.T) {
	t.Parallel()

	nodeAddress := network.NewNodeAddress(network.NamespaceName, network.NodeAddressDefaultID)
	nodeAddress.TypedSpec().Addresses = []netip.Prefix{netip.MustParsePrefix("33.11.22.44/32")}

	hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameStatus.TypedSpec().Hostname = "node1\n6.6.6.6"
	hostnameStatus.TypedSpec().Domainname = "registry.k8s.io"

	valid := networkcfg.NewStaticHostConfigV1Alpha1("10.0.0.1")
	valid.Hostnames = []string{"a", "b\n6.6.6.6 registry.k8s.io", "My_Host"}

	invalid := networkcfg.NewStaticHostConfigV1Alpha1("10.0.0.2\n6.6.6.6")
	invalid.Hostnames = []string{"registry.k8s.io"}

	allInvalid := networkcfg.NewStaticHostConfigV1Alpha1("10.0.0.3")
	allInvalid.Hostnames = []string{"c d"}

	cfg, err := container.New(valid, invalid, allInvalid)
	require.NoError(t, err)

	hosts, err := etcrender.Hosts(hostnameStatus, nodeAddress, cfg)
	require.NoError(t, err)

	assert.Equal(
		t,
		"127.0.0.1 localhost\n::1       localhost ip6-localhost ip6-loopback\nff02::1   ip6-allnodes\nff02::2   ip6-allrouters\n10.0.0.1  a My_Host\n",
		string(hosts),
	)

	// valid unusual hostname is kept as is
	hostnameStatus.TypedSpec().Hostname = "My_Node"
	hostnameStatus.TypedSpec().Domainname = ""

	hosts, err = etcrender.Hosts(hostnameStatus, nodeAddress, nil)
	require.NoError(t, err)

	assert.Equal(
		t,
		"127.0.0.1   localhost\n33.11.22.44 My_Node\n::1         localhost ip6-localhost ip6-loopback\nff02::1     ip6-allnodes\nff02::2     ip6-allrouters\n",
		string(hosts),
	)
}
