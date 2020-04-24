// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"fmt"

	"github.com/talos-systems/talos/api/machine"
)

// Sequence represents a sequence type.
type Sequence int

const (
	// SequenceBoot is the boot sequence.
	SequenceBoot Sequence = iota
	// SequenceInitialize is the initialize sequence.
	SequenceInitialize
	// SequenceInstall is the install sequence.
	SequenceInstall
	// SequenceShutdown is the shutdown sequence.
	SequenceShutdown
	// SequenceUpgrade is the upgrade sequence.
	SequenceUpgrade
	// SequenceReset is the reset sequence.
	SequenceReset
	// SequenceReboot is the reboot sequence.
	SequenceReboot
	// SequenceNoop is the noop sequence.
	SequenceNoop
)

const (
	boot       = "boot"
	initialize = "initialize"
	install    = "install"
	shutdown   = "shutdown"
	upgrade    = "upgrade"
	reset      = "reset"
	reboot     = "reboot"
	noop       = "noop"
)

// String returns the string representation of a `Sequence`.
func (s Sequence) String() string {
	return [...]string{boot, initialize, install, shutdown, upgrade, reset, reboot, noop}[s]
}

// ParseSequence returns a `Sequence` that matches the specified string.
func ParseSequence(s string) (seq Sequence, err error) {
	switch s {
	case boot:
		seq = SequenceBoot
	case initialize:
		seq = SequenceInitialize
	case install:
		seq = SequenceInstall
	case shutdown:
		seq = SequenceShutdown
	case upgrade:
		seq = SequenceUpgrade
	case reset:
		seq = SequenceReset
	case reboot:
		seq = SequenceReboot
	case noop:
		seq = SequenceNoop
	default:
		return seq, fmt.Errorf("unknown runtime sequence: %q", s)
	}

	return seq, nil
}

// Sequencer describes the set of sequences required for the lifecycle
// management of the operating system.
type Sequencer interface {
	Boot(Runtime) []Phase
	Initialize(Runtime) []Phase
	Install(Runtime) []Phase
	Reboot(Runtime) []Phase
	Reset(Runtime, *machine.ResetRequest) []Phase
	Shutdown(Runtime) []Phase
	Upgrade(Runtime, *machine.UpgradeRequest) []Phase
}
