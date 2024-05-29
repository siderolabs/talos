// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build 386

package ntp

import (
	"time"

	"golang.org/x/sys/unix"
)

func toTimeval(offset time.Duration) unix.Timeval {
	t := unix.Timeval{
		Sec:  int32(offset / time.Second),
		Usec: int32(offset / time.Nanosecond % time.Second),
	}

	// kernel wants tv_usec to be positive
	if t.Usec < 0 {
		t.Sec--
		t.Usec += int32(time.Second / time.Nanosecond)
	}

	return t
}

func toUnix(ts unix.Timespec) time.Time {
	return time.Unix(int64(ts.Sec), int64(ts.Nsec))
}

func setOffset(t *unix.Timex, offset time.Duration) {
	t.Offset = int32(offset / time.Nanosecond)
}

func setConstant(t *unix.Timex, constant int) { t.Constant = int32(constant) }
