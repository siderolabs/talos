// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dhcpparse

import (
	"strings"

	"github.com/insomniacslk/dhcp/dhcpv6"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ParseDHCP6FQDN converts a DHCPv6 FQDN option into hostname specs.
//
// It returns nil if the option is missing, or if the hostname contains whitespace or
// control characters, as they would allow to inject extra content into /etc/hosts and /etc/resolv.conf.
func ParseDHCP6FQDN(fqdn *dhcpv6.OptFQDN) []network.HostnameSpecSpec {
	if fqdn == nil || fqdn.DomainName == nil || len(fqdn.DomainName.Labels) == 0 {
		return nil
	}

	spec := network.HostnameSpecSpec{
		Hostname:    fqdn.DomainName.Labels[0],
		Domainname:  strings.Join(fqdn.DomainName.Labels[1:], "."),
		ConfigLayer: network.ConfigOperator,
	}

	if spec.ValidateChars() != nil {
		return nil
	}

	return []network.HostnameSpecSpec{spec}
}
