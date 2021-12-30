// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import (
	"github.com/jsimonetti/rtnetlink"
)

//go:generate enumer -type=OperationalState -linecomment -text

// OperationalState wraps rtnetlink.OperationalState for YAML marshaling.
type OperationalState uint8

// Constants copied from rtnetlink to provide Stringer interface.
const (
	OperStateUnknown        OperationalState = OperationalState(rtnetlink.OperStateUnknown)        // unknown
	OperStateNotPresent     OperationalState = OperationalState(rtnetlink.OperStateNotPresent)     // notPresent
	OperStateDown           OperationalState = OperationalState(rtnetlink.OperStateDown)           // down
	OperStateLowerLayerDown OperationalState = OperationalState(rtnetlink.OperStateLowerLayerDown) // lowerLayerDown
	OperStateTesting        OperationalState = OperationalState(rtnetlink.OperStateTesting)        // testing
	OperStateDormant        OperationalState = OperationalState(rtnetlink.OperStateDormant)        // dormant
	OperStateUp             OperationalState = OperationalState(rtnetlink.OperStateUp)             // up
)
