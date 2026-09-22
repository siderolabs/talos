// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

import "fmt"

// PowerState is the power state a virtual machine is driven towards.
//
// `running` starts the virtual machine, `stopped` stops it, and `suspended` suspends it.
// `suspended` is reachable only from `running`: a stopped machine is not started in order to
// suspend it.
type PowerState int

// PowerState constants.
//
// PowerStateUnknown stands for "unset", so that a required field left out of a document is
// distinguishable from one naming a real state. It is a member of the enum only because the
// generated protobuf needs one at 0; a document may not name it.
//
//structprotogen:gen_enum
const (
	PowerStateUnknown   PowerState = iota // unknown
	PowerStateRunning                     // running
	PowerStateStopped                     // stopped
	PowerStateSuspended                   // suspended
)

// MarshalText implements encoding.TextMarshaler.
func (i PowerState) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
//
// The text methods are hand-written rather than generated with enumer's -text, so that
// PowerStateUnknown is rejected: it marks a field that was never set, not a state a document may
// ask for.
func (i *PowerState) UnmarshalText(text []byte) error {
	parsed, err := PowerStateString(string(text))
	if err != nil {
		return err
	}

	if parsed == PowerStateUnknown {
		return fmt.Errorf("%s does not belong to PowerState values", text)
	}

	*i = parsed

	return nil
}
