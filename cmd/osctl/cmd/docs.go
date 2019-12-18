// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func filePrepender(filename string) string {
	return "<!-- markdownlint-disable -->\n"
}

func linkHandler(s string) string { return s }

// docsCmd represents the docs command
var docsCmd = &cobra.Command{
	Use:    "docs <output>",
	Short:  "Generate documentation for the CLI",
	Long:   ``,
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := args[0]

		if err := os.MkdirAll(out, 0777); err != nil {
			return fmt.Errorf("failed to create output directory %q", out)
		}

		if err := doc.GenMarkdownTreeCustom(rootCmd, out, filePrepender, linkHandler); err != nil {
			return fmt.Errorf("failed to generate docs: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(docsCmd)
}
