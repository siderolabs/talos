// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

// PCIDriverRebindPhase describes where a device stands in its rebind to a target driver.
type PCIDriverRebindPhase int

// PCIDriverRebind phases.
//
// Bound, Unbound and Releasing are the phases of a device the controller took from its driver: only
// those are handed back to their default driver once their configuration is removed.
//
//structprotogen:gen_enum
const (
	PCIDriverRebindPhasePending   PCIDriverRebindPhase = iota // pending
	PCIDriverRebindPhaseHeld                                  // held
	PCIDriverRebindPhaseAdopted                               // adopted
	PCIDriverRebindPhaseBound                                 // bound
	PCIDriverRebindPhaseUnbound                               // unbound
	PCIDriverRebindPhaseReleasing                             // releasing
	PCIDriverRebindPhaseFailed                                // failed
)

// Moved reports whether the controller took the device from its driver in this phase, so that it is
// handed back to its default driver once its configuration is removed.
func (p PCIDriverRebindPhase) Moved() bool {
	switch p { //nolint:exhaustive
	case PCIDriverRebindPhaseBound, PCIDriverRebindPhaseUnbound, PCIDriverRebindPhaseReleasing:
		return true
	default:
		return false
	}
}
