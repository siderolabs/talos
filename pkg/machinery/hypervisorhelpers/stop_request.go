// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

import "strings"

// VirtualMachineStopRequest encodes a stop request as "<mode>/<id>".
//
// The ID is never reused, so a stop asked for again is told apart from the one before it.
func VirtualMachineStopRequest(mode StopMode, id string) string {
	return mode.String() + "/" + id
}

// VirtualMachineStopRequestMode decodes the mode of a stop request, and StopModeUnknown for a value
// which is not one.
func VirtualMachineStopRequestMode(request string) StopMode {
	name, _, ok := strings.Cut(request, "/")
	if !ok {
		return StopModeUnknown
	}

	mode, err := StopModeString(name)
	if err != nil {
		return StopModeUnknown
	}

	return mode
}
