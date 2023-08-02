// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package timex provides a simple wrapper around adjtimex syscall.
package timex

import (
	"strings"

	"golang.org/x/sys/unix"
)

// Status is bitmask field of statuses.
type Status int32

func (status Status) String() string {
	var labels []string

	for _, item := range []struct {
		bit   Status
		label string
	}{
		{unix.STA_PLL, "STA_PLL"},             /* enable PLL updates (rw) */
		{unix.STA_PPSFREQ, "STA_PPSFREQ"},     /* enable PPS freq discipline (rw) */
		{unix.STA_PPSTIME, "STA_PPSTIME"},     /* enable PPS time discipline (rw) */
		{unix.STA_FLL, "STA_FLL"},             /* select frequency-lock mode (rw) */
		{unix.STA_INS, "STA_INS"},             /* insert leap (rw) */
		{unix.STA_DEL, "STA_DEL"},             /* delete leap (rw) */
		{unix.STA_UNSYNC, "STA_UNSYNC"},       /* clock unsynchronized (rw) */
		{unix.STA_FREQHOLD, "STA_FREQHOLD"},   /* hold frequency (rw) */
		{unix.STA_PPSSIGNAL, "STA_PPSSIGNAL"}, /* PPS signal present (ro) */
		{unix.STA_PPSJITTER, "STA_PPSJITTER"}, /* PPS signal jitter exceeded (ro) */
		{unix.STA_PPSWANDER, "STA_PPSWANDER"}, /* PPS signal wander exceeded (ro) */
		{unix.STA_PPSERROR, "STA_PPSERROR"},   /* PPS signal calibration error (ro) */
		{unix.STA_CLOCKERR, "STA_CLOCKERR"},   /* clock hardware fault (ro) */
		{unix.STA_NANO, "STA_NANO"},           /* resolution (0 = us, 1 = ns) (ro) */
		{unix.STA_MODE, "STA_MODE"},           /* mode (0 = PLL, 1 = FLL) (ro) */
		{unix.STA_CLK, "STA_CLK"},             /* clock source (0 = A, 1 = B) (ro) */
	} {
		if (status & item.bit) == item.bit {
			labels = append(labels, item.label)
		}
	}

	return strings.Join(labels, " | ")
}

// State is clock state.
type State int

func (state State) String() string {
	switch state {
	case unix.TIME_OK:
		return "TIME_OK"
	case unix.TIME_INS:
		return "TIME_INS"
	case unix.TIME_DEL:
		return "TIME_DEL"
	case unix.TIME_OOP:
		return "TIME_OOP"
	case unix.TIME_WAIT:
		return "TIME_WAIT"
	case unix.TIME_ERROR:
		return "TIME_ERROR"
	default:
		return "TIME_UNKNOWN"
	}
}

// Adjtimex provides a wrapper around syscall.Adjtimex.
func Adjtimex(buf *unix.Timex) (state State, err error) {
	st, err := unix.ClockAdjtime(unix.CLOCK_REALTIME, buf)

	return State(st), err
}
