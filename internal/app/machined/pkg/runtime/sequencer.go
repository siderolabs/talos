// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"fmt"

	"github.com/talos-systems/talos/pkg/machinery/api/machine"
)

// Sequence represents a sequence type.
type Sequence int

const (
	// SequenceBoot is the boot sequence.
	SequenceBoot Sequence = iota
	// SequenceBootstrap is the boot sequence.
	SequenceBootstrap
	// SequenceInitialize is the initialize sequence.
	SequenceInitialize
	// SequenceInstall is the install sequence.
	SequenceInstall
	// SequenceShutdown is the shutdown sequence.
	SequenceShutdown
	// SequenceUpgrade is the upgrade sequence.
	SequenceUpgrade
	// SequenceStageUpgrade is the stage upgrade sequence.
	SequenceStageUpgrade
	// SequenceReset is the reset sequence.
	SequenceReset
	// SequenceReboot is the reboot sequence.
	SequenceReboot
	// SequenceNoop is the noop sequence.
	SequenceNoop
)

const (
	boot         = "boot"
	bootstrap    = "bootstrap"
	initialize   = "initialize"
	install      = "install"
	shutdown     = "shutdown"
	upgrade      = "upgrade"
	stageUpgrade = "stageUpgrade"
	reset        = "reset"
	reboot       = "reboot"
	noop         = "noop"
)

// String returns the string representation of a `Sequence`.
func (s Sequence) String() string {
	return [...]string{boot, bootstrap, initialize, install, shutdown, upgrade, stageUpgrade, reset, reboot, noop}[s]
}

// ParseSequence returns a `Sequence` that matches the specified string.
//
//nolint:gocyclo
func ParseSequence(s string) (seq Sequence, err error) {
	switch s {
	case boot:
		seq = SequenceBoot
	case bootstrap:
		seq = SequenceBootstrap
	case initialize:
		seq = SequenceInitialize
	case install:
		seq = SequenceInstall
	case shutdown:
		seq = SequenceShutdown
	case upgrade:
		seq = SequenceUpgrade
	case stageUpgrade:
		seq = SequenceStageUpgrade
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

// ResetOptions are parameters to Reset sequence.
type ResetOptions interface {
	GetGraceful() bool
	GetReboot() bool
	GetSystemDiskTargets() []PartitionTarget
}

// PartitionTarget provides interface to the disk partition.
type PartitionTarget interface {
	fmt.Stringer
	Format() error
}

// Sequencer describes the set of sequences required for the lifecycle
// management of the operating system.
type Sequencer interface {
	Boot(Runtime) []Phase
	Bootstrap(Runtime) []Phase
	Initialize(Runtime) []Phase
	Install(Runtime) []Phase
	Reboot(Runtime) []Phase
	Reset(Runtime, ResetOptions) []Phase
	Shutdown(Runtime) []Phase
	StageUpgrade(Runtime, *machine.UpgradeRequest) []Phase
	Upgrade(Runtime, *machine.UpgradeRequest) []Phase
}

// EventSequenceStart represents the sequence start event.
type EventSequenceStart struct {
	Sequence Sequence
}

// EventFatalSequencerError represents a fatal sequencer error.
type EventFatalSequencerError struct {
	Error    error
	Sequence Sequence
}
