// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package common provides common functionality for talosctl commands.
//
//nolint:revive
package common

import "fmt"

// SuppressErrors is a flag to suppress printing errors after the command was run.
var SuppressErrors = false

// ExitError makes talosctl exit with the given code, without printing anything, e.g. to pass on the
// exit code of a command run on the node.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}
