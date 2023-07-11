// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tpm2 provides TPM2.0 related functionality helpers.
package tpm2

// Phase is the phase value extended to the PCR.
// TODO: once ukify is part of installer refer from here.
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
