// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package cluster

import (
	"fmt"
	"net/netip"

	"github.com/siderolabs/siderolink/pkg/wireguard"
)

type nodeAddrGenerator struct {
	prefix   netip.Prefix
	nodeAddr netip.Addr
}

func makeNodeAddrGenerator() nodeAddrGenerator {
	prefix := wireguard.NetworkPrefix("")
	nodeAddr := prefix.Addr().Next()

	return nodeAddrGenerator{
		prefix:   prefix,
		nodeAddr: nodeAddr,
	}
}

func (ng *nodeAddrGenerator) GenerateRandomNodeAddr() (netip.Addr, error) {
	result, err := wireguard.GenerateRandomNodeAddr(ng.prefix)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("failed to generate random node address: %w", err)
	}

	return result.Addr(), nil
}

func (ng *nodeAddrGenerator) GetAgentNodeAddr() string {
	return ng.nodeAddr.String()
}
