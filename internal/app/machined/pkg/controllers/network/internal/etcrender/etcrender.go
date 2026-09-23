// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package etcrender contains helpers which format /etc network files, e.g. /etc/hosts.
package etcrender

import (
	"bytes"
	"fmt"
	"iter"
	"maps"
	"net/netip"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ResolvConf renders /etc/resolv.conf file content based on the provided nameservers and search domains.
func ResolvConf(nameservers iter.Seq2[int, netip.Addr], searchDomains []string) []byte {
	var buf bytes.Buffer

	for i, ns := range nameservers {
		if i >= 3 {
			// only use first 3 nameservers, see MAXNS in https://linux.die.net/man/5/resolv.conf
			break
		}

		fmt.Fprintf(&buf, "nameserver %s\n", ns)
	}

	// search domains might come from untrusted sources (e.g. DHCP), so skip the ones
	// which would inject extra content into resolv.conf
	searchDomains = xslices.Filter(searchDomains, func(domain string) bool {
		return nethelpers.ValidateDNSNameChars(domain) == nil
	})

	if len(searchDomains) > 0 {
		fmt.Fprintf(&buf, "\nsearch %s\n", strings.Join(searchDomains, " "))
	}

	return buf.Bytes()
}

// Hosts renders /etc/hosts file content based on the provided hostname and node address status, as well as any additional host configurations from the config provider.
//
//nolint:gocyclo
func Hosts(hostnameStatus *network.HostnameStatus, nodeAddressStatus *network.NodeAddress, cfgProvider config.Config) ([]byte, error) {
	var buf bytes.Buffer

	tabW := tabwriter.NewWriter(&buf, 0, 0, 1, ' ', 0)

	write := func(s string) { tabW.Write([]byte(s)) } //nolint:errcheck

	write("127.0.0.1\tlocalhost\n")

	// hostname might come from untrusted sources (e.g. DHCP), so skip it if it would inject extra content into hosts file
	if nodeAddressStatus != nil && hostnameStatus != nil &&
		nethelpers.ValidateDNSNameChars(hostnameStatus.TypedSpec().FQDN()) == nil {
		write(fmt.Sprintf("%s\t%s", nodeAddressStatus.TypedSpec().Addresses[0].Addr(), hostnameStatus.TypedSpec().FQDN()))

		if hostnameStatus.TypedSpec().Hostname != hostnameStatus.TypedSpec().FQDN() {
			write(" " + hostnameStatus.TypedSpec().Hostname)
		}

		write("\n")
	}

	write("::1\tlocalhost ip6-localhost ip6-loopback\n")
	write("ff02::1\tip6-allnodes\n")
	write("ff02::2\tip6-allrouters\n")

	hostMap := map[string][]string{}

	if cfgProvider != nil {
		for _, extraHost := range cfgProvider.NetworkStaticHostConfig() {
			if nethelpers.ValidateDNSNameChars(extraHost.IP()) != nil {
				continue
			}

			hostMap[extraHost.IP()] = append(hostMap[extraHost.IP()], xslices.Filter(extraHost.Aliases(), func(alias string) bool {
				return alias != "" && nethelpers.ValidateDNSNameChars(alias) == nil
			})...)
		}
	}

	for _, addr := range slices.Sorted(maps.Keys(hostMap)) {
		if len(hostMap[addr]) == 0 {
			continue
		}

		write(fmt.Sprintf("%s\t%s\n", addr, strings.Join(hostMap[addr], " ")))
	}

	if err := tabW.Flush(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
