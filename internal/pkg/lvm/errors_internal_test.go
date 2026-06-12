// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvm

import (
	"errors"
	"testing"
)

func TestMatchStderr(t *testing.T) {
	for _, test := range []struct {
		name string
		out  string
		want error
	}{
		{
			name: "device partitioned",
			out:  "  Cannot use /dev/vda: device is partitioned",
			want: ErrDevicePartitioned,
		},
		{
			name: "vg already exists",
			out:  "  A volume group called vg-pool already exists.",
			want: ErrExists,
		},
		{
			name: "lv already exists",
			out:  "  Logical volume lv-data already exists in Volume group vg-pool.",
			want: ErrExists,
		},
		{
			name: "pv already in vg",
			out:  "  Physical volume '/dev/sda1' is already in volume group 'vg-pool'",
			want: ErrExists,
		},
		{
			name: "pv already initialized",
			out:  "  Can't initialize PV '/dev/sda1' without -ff.",
			want: ErrExists,
		},
		{
			name: "vg not empty",
			out:  `Volume group "vg0" still contains 2 logical volume(s)`,
			want: ErrNotEmpty,
		},
		{
			name: "pv in use",
			out:  "PV /dev/sda1 is used by VG vg0 so please use vgreduce first.",
			want: ErrInUse,
		},
		{
			name: "not found",
			out:  `Volume group "vg0" not found`,
			want: ErrNotFound,
		},
		{
			name: "unmatched",
			out:  "some other failure",
			want: nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := matchStderr([]byte(test.out))
			if !errors.Is(got, test.want) {
				t.Fatalf("matchStderr(%q) = %v, want %v", test.out, got, test.want)
			}
		})
	}
}
