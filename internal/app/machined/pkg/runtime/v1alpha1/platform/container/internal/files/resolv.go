// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"bytes"
	"net/netip"
	"os"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ReadResolvConf reads and parses /etc/resolv.conf file.
func ReadResolvConf(path string) (network.ResolverSpecSpec, error) {
	resolverSpec := network.ResolverSpecSpec{
		ConfigLayer: network.ConfigPlatform,
	}

	resolvers, err := os.ReadFile(path)
	if err != nil {
		return resolverSpec, err
	}

	for _, line := range bytes.Split(resolvers, []byte("\n")) {
		line = bytes.TrimSpace(line)
		line, _, _ = bytes.Cut(line, []byte("#"))

		if !bytes.HasPrefix(line, []byte("nameserver")) {
			continue
		}

		line = bytes.TrimSpace(bytes.TrimPrefix(line, []byte("nameserver")))

		if addr, err := netip.ParseAddr(string(line)); err == nil {
			resolverSpec.DNSServers = append(resolverSpec.DNSServers, addr)
		}
	}

	return resolverSpec, nil
}
