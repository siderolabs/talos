// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package constants

type Section string

const (
	Linux   Section = ".linux"
	OSRel   Section = ".osrel"
	CMDLine Section = ".cmdline"
	Initrd  Section = ".initrd"
	Splash  Section = ".splash"
	DTB     Section = ".dtb"
	Uname   Section = ".uname"
	SBAT    Section = ".sbat"
	PCRSig  Section = ".pcrsig"
	PCRPKey Section = ".pcrpkey"
)

// derived from https://github.com/systemd/systemd/blob/main/src/fundamental/tpm-pcr.h#L23-L36
// OrderedSections returns the sections that are measured into PCR
// .pcrsig section is omitted here since that's what we are calulating here
func OrderedSections() []Section {
	// DO NOT REARRANGE
	return []Section{Linux, OSRel, CMDLine, Initrd, Splash, DTB, Uname, SBAT, PCRPKey}
}

type Phase string

const (
	// EnterInitrd is the phase value extended to the PCR during the initrd.
	EnterInitrd Phase = "enter-initrd"
	// LeaveInitrd is the phase value extended to the PCR just before switching to machined.
	LeaveInitrd Phase = "leave-initrd"
	// EnterMachined is the phase value extended to the PCR before starting machined.
	// The should be only a signed signature for the enter-machined phase.
	EnterMachined Phase = "enter-machined"
	// StartTheWorld is the phase value extended to the PCR before starting all services.
	StartTheWorld Phase = "start-the-world"
)

type PhaseInfo struct {
	Phase              Phase
	CalculateSignature bool
}

// derived from https://github.com/systemd/systemd/blob/v253/src/boot/measure.c#L295-L308
// ref: https://www.freedesktop.org/software/systemd/man/systemd-pcrphase.service.html#Description
// In the case of Talos disk decryption, happens in machined, so we need to only sign EnterMachined
// so that machined can only decrypt the disk if the system booted with the correct kernel/initrd/cmdline
// OrderedPhases returns the phases that are measured
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

const (
	// UKI sections except `.pcrsig` are measured into PCR 11 by sd-stub
	UKIPCR = 11
)
