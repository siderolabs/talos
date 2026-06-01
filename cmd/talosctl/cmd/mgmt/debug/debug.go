// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package debug implements "debug" subcommands.
package debug

import (
	"github.com/spf13/cobra"
)

// Cmd represents the debug command.
var Cmd = &cobra.Command{
	Use:    "debug-tool",
	Short:  "A collection of commands to facilitate debugging of Talos.",
	Hidden: true,
	Long:   ``,
}
