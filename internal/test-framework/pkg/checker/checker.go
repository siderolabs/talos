// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package checker

import (
	"os/exec"
	"strings"
	"time"
)

// Check represents a check/test that should be run against
// the specified command. The stdout of the command is passed
// into the Check to validate if the command was successful
// or not.
type Check struct {
	Command *exec.Cmd
	Check   Checkfn
	Name    string
	Wait    time.Duration
	Stdout  strings.Builder
	Stderr  strings.Builder
}

// Checkfn is a function to validate the output is appropriate.
type Checkfn func(string) bool
