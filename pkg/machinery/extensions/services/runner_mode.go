// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

//go:generate go tool github.com/dmarkham/enumer -type=RunnerMode -linecomment -text

// RunnerMode specifies how the extension service should be run.
type RunnerMode int

// RunnerMode constants.
const (
	RunnerModeContainer RunnerMode = iota // container
	RunnerModeHost                        // host
)
