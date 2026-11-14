// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/mount/v3"
)

func TestAttrChanges(t *testing.T) {
	type change struct {
		on    bool
		flags uint64
	}

	for _, tc := range []struct {
		name    string
		changes []change
		want    mount.AttrChanges
	}{
		{name: "none"},
		{
			name:    "read-only",
			changes: []change{{true, unix.MOUNT_ATTR_RDONLY}},
			want:    mount.AttrChanges{Set: unix.MOUNT_ATTR_RDONLY},
		},
		{
			name:    "latest change wins",
			changes: []change{{true, unix.MOUNT_ATTR_RDONLY}, {false, unix.MOUNT_ATTR_RDONLY}},
			want:    mount.AttrChanges{Clr: unix.MOUNT_ATTR_RDONLY},
		},
		{
			name: "independent flags",
			changes: []change{
				{false, unix.MOUNT_ATTR_NOSUID | unix.MOUNT_ATTR_NODEV},
				{true, unix.MOUNT_ATTR_NOEXEC},
				{true, unix.MOUNT_ATTR_NOSUID | unix.MOUNT_ATTR_NODEV},
			},
			want: mount.AttrChanges{Set: unix.MOUNT_ATTR_NOSUID | unix.MOUNT_ATTR_NODEV | unix.MOUNT_ATTR_NOEXEC},
		},
		{
			name:    "noatime replaces the atime mode",
			changes: []change{{true, unix.MOUNT_ATTR_NOATIME}},
			want:    mount.AttrChanges{Set: unix.MOUNT_ATTR_NOATIME, Clr: unix.MOUNT_ATTR__ATIME},
		},
		{
			name:    "clearing noatime restores relatime",
			changes: []change{{true, unix.MOUNT_ATTR_NOATIME}, {false, unix.MOUNT_ATTR_NOATIME}},
			want:    mount.AttrChanges{Clr: unix.MOUNT_ATTR__ATIME},
		},
		{
			name:    "atime mode kept across other changes",
			changes: []change{{true, unix.MOUNT_ATTR_NOATIME}, {false, unix.MOUNT_ATTR_NOEXEC}},
			want:    mount.AttrChanges{Set: unix.MOUNT_ATTR_NOATIME, Clr: unix.MOUNT_ATTR__ATIME | unix.MOUNT_ATTR_NOEXEC},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c mount.AttrChanges

			for _, ch := range tc.changes {
				c.Record(ch.on, ch.flags)
			}

			assert.Equal(t, tc.want, c)
			assert.Equal(t, tc.want == mount.AttrChanges{}, c.Empty())
		})
	}
}
