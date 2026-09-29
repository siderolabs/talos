// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var genCmdFlags struct {
	force bool
}

// Cmd represents the `gen` command.
var Cmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate base secrets, machine configuration, and other files.",
	Long:  ``,
}

func init() {
	Cmd.PersistentFlags().BoolVarP(&genCmdFlags.force, "force", "f", false, "will overwrite existing files")
}

func validateFileExists(file string) error {
	if !genCmdFlags.force {
		if _, err := os.Stat(file); err == nil {
			return fmt.Errorf("file %q already exists, use --force to overwrite", file)
		}
	}

	return nil
}
