// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// Sequence represents a sequence type.
type Sequence int

const (
	// SequenceNoop is the noop sequence.
	SequenceNoop Sequence = iota
	// SequenceBoot is the boot sequence.
	SequenceBoot
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
	// SequenceMaintenanceUpgrade is the upgrade sequence in maintenance mode.
	SequenceMaintenanceUpgrade
	// SequenceReset is the reset sequence.
	SequenceReset
	// SequenceReboot is the reboot sequence.
	SequenceReboot
)

const (
	boot               = "boot"
	initialize         = "initialize"
	install            = "install"
	shutdown           = "shutdown"
	upgrade            = "upgrade"
	stageUpgrade       = "stageUpgrade"
	maintenanceUpgrade = "maintenanceUpgrade"
	reset              = "reset"
	reboot             = "reboot"
	noop               = "noop"
)

var sequenceTakeOver = map[Sequence]map[Sequence]struct{}{
	SequenceInitialize: {
		SequenceMaintenanceUpgrade: {},
	},
	SequenceBoot: {
		SequenceReboot:  {},
		SequenceReset:   {},
		SequenceUpgrade: {},
	},
	SequenceReboot: {
		SequenceReboot: {},
	},
	SequenceReset: {
		SequenceReboot: {},
	},
}

// String returns the string representation of a `Sequence`.
func (s Sequence) String() string {
	return [...]string{noop, boot, initialize, install, shutdown, upgrade, stageUpgrade, maintenanceUpgrade, reset, reboot}[s]
}

// CanTakeOver defines sequences priority.
//
// | what is running (columns) what is requested (rows) | boot | reboot | reset | upgrade |
// |----------------------------------------------------|------|--------|-------|---------|
// | reboot                                             | Y    | Y      | Y     | N       |
// | reset                                              | Y    | N      | N     | N       |
// | upgrade                                            | Y    | N      | N     | N       |.
func (s Sequence) CanTakeOver(running Sequence) bool {
	if running == SequenceNoop {
		return true
	}

	if sequences, ok := sequenceTakeOver[running]; ok {
		if _, ok = sequences[s]; ok {
			return true
		}
	}

	return false
}

// ParseSequence returns a `Sequence` that matches the specified string.
//
//nolint:gocyclo
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
	case stageUpgrade:
		seq = SequenceStageUpgrade
	case maintenanceUpgrade:
		seq = SequenceMaintenanceUpgrade
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
	GetMode() machine.ResetRequest_WipeMode
	GetUserDisksToWipe() []string
	GetSystemDiskTargets() []PartitionTarget
}

// PartitionTarget provides interface to the disk partition.
type PartitionTarget interface {
	Wipe(context.Context, func(string, ...any)) error
	GetLabel() string
}

// Sequencer describes the set of sequences required for the lifecycle
// management of the operating system.
type Sequencer interface {
	Boot(Runtime) []Phase
	Initialize(Runtime) []Phase
	Install(Runtime) []Phase
	Reboot(Runtime) []Phase
	Reset(Runtime, ResetOptions) []Phase
	Shutdown(Runtime, *machine.ShutdownRequest) []Phase
	StageUpgrade(Runtime, *machine.UpgradeRequest) []Phase
	Upgrade(Runtime, *machine.UpgradeRequest) []Phase
	MaintenanceUpgrade(Runtime, *machine.UpgradeRequest) []Phase
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
