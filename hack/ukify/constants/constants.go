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
	PCRSig  Section = ".pcrsig"
	PCRPKey Section = ".pcrpkey"
)

// derived from https://github.com/systemd/systemd/blob/main/src/fundamental/tpm-pcr.h#L23-L36
// OrderedSections returns the sections that are measured into PCR
// .pcrsig section is omitted here since that's what we are calulating here
func OrderedSections() []Section {
	// DO NOT REARRANGE
	return []Section{Linux, OSRel, CMDLine, Initrd, Splash, DTB, Uname, PCRPKey}
}

type Phase string

const (
	EnterInitrd Phase = "enter-initrd"
	LeaveInitrd Phase = "leave-initrd"
	SysInit     Phase = "sysinit"
	Ready       Phase = "ready"
)

// derived from https://github.com/systemd/systemd/blob/v253/src/boot/measure.c#L295-L308
// ref: https://www.freedesktop.org/software/systemd/man/systemd-pcrphase.service.html#Description
// OrderedPhases returns the phases that are measured
func OrderedPhases() []Phase {
	// DO NOT REARRANGE
	return []Phase{EnterInitrd, LeaveInitrd, SysInit, Ready}
}

const (
	// UKI sections except `.pcrsig` are measured into PCR 11 by sd-stub
	UKIPCR = 11
)
