// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

// ContainerState describes where a container is in its lifecycle.
type ContainerState int

// Container states.
//
//structprotogen:gen_enum
const (
	ContainerStatePending  ContainerState = iota // pending
	ContainerStatePulling                        // pulling
	ContainerStateStarting                       // starting
	ContainerStateRunning                        // running
	ContainerStateExited                         // exited
	ContainerStateBackoff                        // backoff
	ContainerStateStopping                       // stopping
)

// Health returns the coarse health summary for a state.
//
// Keeping this mapping in one place means the projection cannot drift between controllers.
func (state ContainerState) Health() ContainerHealth {
	switch state {
	case ContainerStatePending:
		return ContainerHealthPending
	case ContainerStatePulling, ContainerStateStarting:
		return ContainerHealthPulling
	case ContainerStateRunning:
		return ContainerHealthHealthy
	case ContainerStateExited, ContainerStateBackoff:
		return ContainerHealthDegraded
	case ContainerStateStopping:
		// Always healthy here: this method has no access to prior state. StatusController is the
		// one that actually preserves the last meaningful value, by skipping this result for
		// Stopping rather than relying on it.
		return ContainerHealthHealthy
	default:
		return ContainerHealthDegraded
	}
}

// ContainerHealth is the coarse answer to "should I be looking at this container?".
type ContainerHealth int

// Container health values.
//
//structprotogen:gen_enum
const (
	ContainerHealthPending  ContainerHealth = iota // pending
	ContainerHealthPulling                         // pulling
	ContainerHealthHealthy                         // healthy
	ContainerHealthDegraded                        // degraded
)

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

// ContainerInstancePhase describes the state of a container instance's execution.
type ContainerInstancePhase int

// Container instance phases.
//
//structprotogen:gen_enum
const (
	ContainerInstancePhaseCreated    ContainerInstancePhase = iota // created
	ContainerInstancePhaseRunning                                  // running
	ContainerInstancePhaseTerminated                               // terminated
	ContainerInstancePhaseFailed                                   // failed
)

// Done reports whether the instance has finished executing, successfully or not.
func (phase ContainerInstancePhase) Done() bool {
	return phase == ContainerInstancePhaseTerminated || phase == ContainerInstancePhaseFailed
}
