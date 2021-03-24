// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package timex provides a simple wrapper around adjtimex syscall.
package timex

import (
	"strings"
	"syscall"
)

// Values for timex.mode.
//
//nolint:golint,stylecheck,revive
const (
	ADJ_OFFSET    = 0x0001
	ADJ_FREQUENCY = 0x0002
	ADJ_MAXERROR  = 0x0004
	ADJ_ESTERROR  = 0x0008
	ADJ_STATUS    = 0x0010
	ADJ_TIMECONST = 0x0020
	ADJ_TAI       = 0x0080
	ADJ_SETOFFSET = 0x0100
	ADJ_MICRO     = 0x1000
	ADJ_NANO      = 0x2000
	ADJ_TICK      = 0x4000
)

// Status is bitmask field of statuses.
type Status int32

// Clock statuses.
//
//nolint:golint,stylecheck,revive
const (
	STA_PLL       = 0x0001 /* enable PLL updates (rw) */
	STA_PPSFREQ   = 0x0002 /* enable PPS freq discipline (rw) */
	STA_PPSTIME   = 0x0004 /* enable PPS time discipline (rw) */
	STA_FLL       = 0x0008 /* select frequency-lock mode (rw) */
	STA_INS       = 0x0010 /* insert leap (rw) */
	STA_DEL       = 0x0020 /* delete leap (rw) */
	STA_UNSYNC    = 0x0040 /* clock unsynchronized (rw) */
	STA_FREQHOLD  = 0x0080 /* hold frequency (rw) */
	STA_PPSSIGNAL = 0x0100 /* PPS signal present (ro) */
	STA_PPSJITTER = 0x0200 /* PPS signal jitter exceeded (ro) */
	STA_PPSWANDER = 0x0400 /* PPS signal wander exceeded (ro) */
	STA_PPSERROR  = 0x0800 /* PPS signal calibration error (ro) */
	STA_CLOCKERR  = 0x1000 /* clock hardware fault (ro) */
	STA_NANO      = 0x2000 /* resolution (0 = us, 1 = ns) (ro) */
	STA_MODE      = 0x4000 /* mode (0 = PLL, 1 = FLL) (ro) */
	STA_CLK       = 0x8000 /* clock source (0 = A, 1 = B) (ro) */
)

func (status Status) String() string {
	var labels []string

	for bit, label := range map[Status]string{
		STA_PLL:       "STA_PLL",
		STA_PPSFREQ:   "STA_PPSFREQ",
		STA_PPSTIME:   "STA_PPSTIME",
		STA_FLL:       "STA_FLL",
		STA_INS:       "STA_INS",
		STA_DEL:       "STA_DEL",
		STA_UNSYNC:    "STA_UNSYNC",
		STA_FREQHOLD:  "STA_FREQHOLD",
		STA_PPSSIGNAL: "STA_PPSSIGNAL",
		STA_PPSJITTER: "STA_PPSJITTER",
		STA_PPSWANDER: "STA_PPSWANDER",
		STA_PPSERROR:  "STA_PPSERROR",
		STA_CLOCKERR:  "STA_CLOCKERR",
		STA_NANO:      "STA_NANO",
		STA_MODE:      "STA_MODE",
		STA_CLK:       "STA_CLK",
	} {
		if (status & bit) == bit {
			labels = append(labels, label)
		}
	}

	return strings.Join(labels, " | ")
}

// State is clock state.
type State int

// Clock states.
//
//nolint:golint,stylecheck,revive
const (
	TIME_OK State = iota
	TIME_INS
	TIME_DEL
	TIME_OOP
	TIME_WAIT
	TIME_ERROR
)

func (state State) String() string {
	return [...]string{"TIME_OK", "TIME_INS", "TIME_DEL", "TIME_OOP", "TIME_WAIT", "TIME_ERROR"}[int(state)]
}

// Adjtimex provides a wrapper around syscall.Adjtimex.
func Adjtimex(buf *syscall.Timex) (state State, err error) {
	st, err := syscall.Adjtimex(buf)

	return State(st), err
}
