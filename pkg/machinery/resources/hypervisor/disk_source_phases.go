// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

// VirtualMachineDiskPhase describes observed disk attachability, not COSI metadata lifecycle.
type VirtualMachineDiskPhase int

// Disk lifecycle phases. Unknown is never attachable; ObservationUnavailable
// carries transient storage observation loss rather than an ordinary wait/failure.
//
//structprotogen:gen_enum
const (
	VirtualMachineDiskPhaseUnknown                VirtualMachineDiskPhase = iota // unknown
	VirtualMachineDiskPhaseNotReady                                              // notReady
	VirtualMachineDiskPhaseReady                                                 // ready
	VirtualMachineDiskPhaseObservationUnavailable                                // observationUnavailable
)

// ContentLibraryPhase describes observed library availability, not COSI metadata lifecycle.
type ContentLibraryPhase int

// Content library lifecycle phases.
//
//structprotogen:gen_enum
const (
	ContentLibraryPhaseUnknown  ContentLibraryPhase = iota // unknown
	ContentLibraryPhaseNotReady                            // notReady
	ContentLibraryPhaseReady                               // ready
)

// CloudInitPhase describes observed seed availability, not COSI metadata lifecycle.
type CloudInitPhase int

// Cloud-init seed lifecycle phases.
//
//structprotogen:gen_enum
const (
	CloudInitPhaseUnknown  CloudInitPhase = iota // unknown
	CloudInitPhaseNotReady                       // notReady
	CloudInitPhaseReady                          // ready
)
