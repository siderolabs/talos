// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

//go:generate go tool github.com/dmarkham/enumer -type=RestartKind -linecomment -text

// RestartKind specifies how the service should be restarted.
type RestartKind int

// RestartKind constants.
const (
	RestartAlways       RestartKind = 1 // always
	RestartNever        RestartKind = 2 // never
	RestartUntilSuccess RestartKind = 3 // untilSuccess
)
