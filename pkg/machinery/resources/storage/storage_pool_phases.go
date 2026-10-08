// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

// StoragePoolPhase describes observed pool attachability, not COSI metadata lifecycle.
type StoragePoolPhase int

// Storage pool lifecycle phases. Unknown is never attachable; ObservationUnavailable
// is transient loss of daemon observation, distinct from an observed ordinary wait/failure.
//
//structprotogen:gen_enum
const (
	StoragePoolPhaseUnknown                StoragePoolPhase = iota // unknown
	StoragePoolPhaseNotReady                                       // notReady
	StoragePoolPhaseReady                                          // ready
	StoragePoolPhaseObservationUnavailable                         // observationUnavailable
)

// StoragePoolVolumePhase describes observed volume attachability, not COSI metadata lifecycle.
type StoragePoolVolumePhase int

// Storage pool volume lifecycle phases. Ready may coexist with deferred growth or Error.
//
//structprotogen:gen_enum
const (
	StoragePoolVolumePhaseUnknown                StoragePoolVolumePhase = iota // unknown
	StoragePoolVolumePhaseNotReady                                             // notReady
	StoragePoolVolumePhaseReady                                                // ready
	StoragePoolVolumePhaseObservationUnavailable                               // observationUnavailable
)
