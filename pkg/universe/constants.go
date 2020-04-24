// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package universe

const (
	// SystemRunPath is the path to write temporary runtime system related files
	// and directories.
	SystemRunPath = "/run/system"

	// MachineSocketPath is the path to file socket of machine API.
	MachineSocketPath = SystemRunPath + "/machined/machine.sock"
)
