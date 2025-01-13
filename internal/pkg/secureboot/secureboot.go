// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package secureboot contains base definitions for the Secure Boot process.
package secureboot

// Phase is the phase value extended to the PCR.
type Phase string

const (
	// EnterInitrd is the phase value extended to the PCR during the initrd.
	EnterInitrd Phase = "enter-initrd"
	// LeaveInitrd is the phase value extended to the PCR just before switching to machined.
	LeaveInitrd Phase = "leave-initrd"
	// EnterMachined is the phase value extended to the PCR before starting machined.
	// There should be only a signed signature for the enter-machined phase.
	EnterMachined Phase = "enter-machined"
	// StartTheWorld is the phase value extended to the PCR before starting all services.
	StartTheWorld Phase = "start-the-world"
)

// PhaseInfo describes which phase extensions are signed/measured.
type PhaseInfo struct {
	Phase              Phase
	CalculateSignature bool
}

// OrderedPhases returns the phases that are measured, in order.
//
// Derived from https://github.com/systemd/systemd/blob/v253/src/boot/measure.c#L295-L308
// ref: https://www.freedesktop.org/software/systemd/man/systemd-pcrphase.service.html#Description
//
// In the case of Talos disk decryption, happens in machined, so we need to only sign EnterMachined
// so that machined can only decrypt the disk if the system booted with the correct kernel/initrd/cmdline
// OrderedPhases returns the phases that are measured.
func OrderedPhases() []PhaseInfo {
	// DO NOT REARRANGE
	return []PhaseInfo{
		{
			Phase:              EnterInitrd,
			CalculateSignature: false,
		},
		{
			Phase:              LeaveInitrd,
			CalculateSignature: false,
		},
		{
			Phase:              EnterMachined,
			CalculateSignature: true,
		},
	}
}
