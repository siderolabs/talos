// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/provision/providers/qemu"
)

// virtioNetHelpQEMU102 is a trimmed `qemu-system-x86_64 -device virtio-net-pci,help` from QEMU 10.2.1.
const virtioNetHelpQEMU102 = `virtio-net-pci options:
  acpi-index=<uint32>    -  (default: 0)
  addr=<int32>           - Slot and optional function number, example: 06.0 or 06 (default: -1)
  guest_tunnel=<bool>    - on/off (default: on)
  guest_tunnel_csum=<bool> - on/off (default: on)
  host_mtu=<uint16>      -  (default: 0)
  host_tunnel=<bool>     - on/off (default: on)
  host_tunnel_csum=<bool> - on/off (default: on)
  mac=<str>              - Ethernet 6-byte MAC Address, example: 52:54:00:12:34:56
`

// virtioNetHelpQEMU82 is a trimmed `qemu-system-x86_64 -device virtio-net-pci,help` from QEMU 8.2.
const virtioNetHelpQEMU82 = `virtio-net-pci options:
  acpi-index=<uint32>    -  (default: 0)
  addr=<int32>           - Slot and optional function number, example: 06.0 or 06 (default: -1)
  host_mtu=<uint16>      -  (default: 0)
  mac=<str>              - Ethernet 6-byte MAC Address, example: 52:54:00:12:34:56
`

func TestParseDeviceProperties(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		map[string]struct{}{
			"acpi-index":        {},
			"addr":              {},
			"guest_tunnel":      {},
			"guest_tunnel_csum": {},
			"host_mtu":          {},
			"host_tunnel":       {},
			"host_tunnel_csum":  {},
			"mac":               {},
		},
		qemu.ParseDevicePropertiesForTest([]byte(virtioNetHelpQEMU102)),
	)

	assert.Empty(t, qemu.ParseDevicePropertiesForTest(nil))
	assert.Empty(t, qemu.ParseDevicePropertiesForTest([]byte("virtio-net-pci options:\n")))
}

func TestVirtioNetTunnelOffloadArgs(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		",guest_tunnel=off,guest_tunnel_csum=off,host_tunnel=off,host_tunnel_csum=off",
		qemu.VirtioNetTunnelOffloadArgsForTest(qemu.ParseDevicePropertiesForTest([]byte(virtioNetHelpQEMU102))),
	)

	assert.Empty(t, qemu.VirtioNetTunnelOffloadArgsForTest(qemu.ParseDevicePropertiesForTest([]byte(virtioNetHelpQEMU82))))
	assert.Empty(t, qemu.VirtioNetTunnelOffloadArgsForTest(nil))
}
