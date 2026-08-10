// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

// ContainerImagePhase describes the state of a container's image pull.
type ContainerImagePhase int

// Container image phases.
//
//structprotogen:gen_enum
const (
	ContainerImagePhasePending ContainerImagePhase = iota // pending
	ContainerImagePhasePulling                            // pulling
	ContainerImagePhaseReady                              // ready
	ContainerImagePhaseFailed                             // failed
)
