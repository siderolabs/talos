// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/provision/providers/qemu"
)

func TestFabricDevice(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		"e1000,netdev=fabric0,mac=02:00:00:00:00:01",
		qemu.FabricDeviceForTest(false, 0, "02:00:00:00:00:01", 0),
	)

	assert.Equal(t,
		"virtio-net-pci,netdev=fabric1,mac=02:00:00:00:00:01,addr=0x11,host_mtu=1430",
		qemu.FabricDeviceForTest(true, 1, "02:00:00:00:00:01", 1430),
	)
}
