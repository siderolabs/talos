// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "test-framework",
	Short: "A CLI for performing tests",
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}
