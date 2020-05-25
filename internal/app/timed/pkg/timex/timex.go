// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package timex provides simple wrapper around adjtimex syscall.
package timex

import "syscall"

// Values for timex.mode.
//
//nolint: golint, stylecheck
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

// State is clock state.
type State int

// Clock states.
//
//nolint: golint, stylecheck
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
