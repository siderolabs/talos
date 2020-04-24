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
	// Boot is the boot sequence.
	Boot Sequence = iota
	// Initialize is the initialize sequence.
	Initialize
	// Shutdown is the shutdown sequence.
	Shutdown
	// Upgrade is the upgrade sequence.
	Upgrade
	// Reset is the reset sequence.
	Reset
	// Reboot is the reset sequence.
	Reboot
	// Noop is the noop sequence.
	Noop
)

const (
	boot       = "boot"
	initialize = "initialize"
	shutdown   = "shutdown"
	upgrade    = "upgrade"
	reset      = "reset"
	reboot     = "reboot"
	noop       = "noop"
)

// String returns the string representation of a `Sequence`.
func (s Sequence) String() string {
	return [...]string{boot, initialize, shutdown, upgrade, reset, reboot, noop}[s]
}

// ParseSequence returns a `Sequence` that matches the specified string.
func ParseSequence(s string) (seq Sequence, err error) {
	switch s {
	case boot:
		seq = Boot
	case initialize:
		seq = Initialize
	case shutdown:
		seq = Shutdown
	case upgrade:
		seq = Upgrade
	case reset:
		seq = Reset
	case reboot:
		seq = Reboot
	case noop:
		seq = Noop
	default:
		return seq, fmt.Errorf("unknown runtime sequence: %q", s)
	}

	return seq, nil
}

// Sequencer describes the set of sequences required for the lifecycle
// management of the operating system.
type Sequencer interface {
	Boot() []Phase
	Initialize() []Phase
	Reboot() []Phase
	Reset(*machine.ResetRequest) []Phase
	Shutdown() []Phase
	Upgrade(*machine.UpgradeRequest) []Phase
}
