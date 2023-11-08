// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"fmt"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
)

const (
	crtExt = ".crt"
	keyExt = ".key"
)

var genCmdFlags struct {
	force bool
}

// Cmd represents the `gen` command.
var Cmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate CAs, certificates, and private keys",
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

func validateFilesExists(files []string) error {
	var combinedErr multierror.Error

	for _, file := range files {
		if err := validateFileExists(file); err != nil {
			combinedErr.Errors = append(combinedErr.Errors, err)
		}
	}

	return combinedErr.ErrorOrNil()
}
