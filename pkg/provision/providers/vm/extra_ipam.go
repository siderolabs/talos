// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"
	"net/netip"

	"github.com/siderolabs/talos/pkg/provision"
)

// DumpExtraIPAMRecords writes the reserved guest records for a virtual network.
func DumpExtraIPAMRecords(statePath string, network provision.NetworkRequest) error {
	for _, record := range network.ExtraDHCPRecords {
		var nameservers []netip.Addr

		// DHCP options must contain addresses in the guest's address family,
		// just like the ordinary QEMU node records.
		for _, nameserver := range network.Nameservers {
			if nameserver.Is4() == record.IP.Addr().Is4() {
				nameservers = append(nameservers, nameserver)
			}
		}

		if err := DumpIPAMRecord(statePath, IPAMRecord{
			IP:          record.IP.Addr(),
			Netmask:     byte(record.IP.Bits()),
			Gateway:     record.Gateway,
			MAC:         record.MAC,
			Hostname:    record.Name,
			MTU:         network.MTU,
			Nameservers: nameservers,
		}); err != nil {
			return fmt.Errorf("error dumping extra IPAM record: %w", err)
		}
	}

	return nil
}
