// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import "github.com/siderolabs/talos/tools/loglinter/internal/loglinter"

// Issue describes a single finding reported by the standalone CLI.
type Issue = loglinter.Issue

// Run executes the standalone linter against the configured repository root.
func Run(configPath string, rawTargets []string) ([]Issue, error) {
	return loglinter.Run(configPath, rawTargets)
}
