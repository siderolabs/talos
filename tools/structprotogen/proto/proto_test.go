// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proto_test

import (
	"testing"

	"github.com/siderolabs/talos/tools/structprotogen/proto"
)

func TestToSnakeCase(t *testing.T) {
	for _, test := range []struct {
		name string
		want string
	}{
		{name: "VLANs", want: "vlans"},
		{name: "VLAN", want: "vlan"},
		{name: "MacVLAN", want: "mac_vlan"},
		{name: "AllowedIPs", want: "allowed_ips"},
		{name: "ExternalIPs", want: "external_ips"},
		{name: "ChassisID", want: "chassis_id"},
		{name: "PortID", want: "port_id"},
		{name: "ManagementAddresses", want: "management_addresses"},
		{name: "MTU", want: "mtu"},
		{name: "URL", want: "url"},
		// Keep the exception exact rather than changing other plural acronyms.
		{name: "OtherVLANs", want: "other_vla_ns"},
		{name: "VLANsEnabled", want: "vla_ns_enabled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := proto.ToSnakeCase(test.name); got != test.want {
				t.Errorf("ToSnakeCase(%q) = %q, want %q", test.name, got, test.want)
			}
		})
	}
}
