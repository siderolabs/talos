// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

// VolumePhase describes volume phase.
type VolumePhase int

// Volume phases.
//
//structprotogen:gen_enum
const (
	VolumePhaseWaiting     VolumePhase = iota // waiting
	VolumePhaseFailed                         // failed
	VolumePhaseMissing                        // missing
	VolumePhaseLocated                        // located
	VolumePhaseProvisioned                    // provisioned
	VolumePhasePrepared                       // prepared
	VolumePhaseReady                          // ready
	VolumePhaseClosed                         // closed
)
