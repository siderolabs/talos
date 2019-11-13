// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

func filePrepender(filename string) string {
	return "<!-- markdownlint-disable -->\n"
}

func linkHandler(s string) string { return s }

// docsCmd represents the docs command
var docsCmd = &cobra.Command{
	Use:   "docs <output>",
	Short: "Generate documentation for the CLI",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			helpers.Fatalf("an output path is required")
		}

		out := args[0]

		if err := os.MkdirAll(out, 0777); err != nil {
			helpers.Fatalf("failed to create output directory %q", out)
		}

		if err := doc.GenMarkdownTreeCustom(rootCmd, out, filePrepender, linkHandler); err != nil {
			helpers.Fatalf("failed to generate docs: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(docsCmd)
}
