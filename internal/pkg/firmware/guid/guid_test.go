// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package guid_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/pkg/firmware/guid"
)

func TestHashString(t *testing.T) {
	// golden values from fwupd's libfwupd/fwupd-common-test.c
	assert.Equal(t, "886313e1-3b8a-5372-9b90-0c9aee199e5d", guid.HashString("python.org"))
	assert.Equal(t, "1fbd1f2c-80f4-5d7c-a6ad-35c7b9bd5486", guid.HashString("8086:0406"))
}

func TestNVMeInstanceIDs(t *testing.T) {
	assert.Equal(t,
		[]string{
			`NVME\VEN_1179&DEV_0115`,
			`NVME\VEN_1179&DEV_0115&SUBSYS_11790001`,
			"THNSN5512GPU7 TOSHIBA",
		},
		guid.NVMeInstanceIDs(0x1179, 0x0115, 0x1179, 0x0001, " THNSN5512GPU7 TOSHIBA "),
	)

	assert.Equal(t,
		[]string{`NVME\VEN_144D&DEV_A804`},
		guid.NVMeInstanceIDs(0x144d, 0xa804, 0, 0, ""),
	)
}
