// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysblock_test

import (
	"testing"

	"github.com/mdlayher/kobject"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/sysblock"
)

func TestWalk(t *testing.T) {
	events, err := sysblock.Walk("/sys/block")
	require.NoError(t, err)

	require.NotEmpty(t, events)

	// there should be at least a single blockdevice and a partition
	partitions, disks := 0, 0

	for _, event := range events {
		require.Equal(t, "block", event.Subsystem)
		require.EqualValues(t, kobject.Add, event.Action)

		require.NotEmpty(t, event.DevicePath)
		require.NotEmpty(t, event.Action)

		switch event.Values["DEVTYPE"] {
		case "partition":
			partitions++
		case "disk":
			disks++
		}
	}

	require.Greater(t, partitions, 0)
	require.Greater(t, disks, 0)
}
