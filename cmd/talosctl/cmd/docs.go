// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	v1alpha1 "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

func filePrepender(filename string) string {
	return "<!-- markdownlint-disable -->\n"
}

func linkHandler(s string) string { return s }

var (
	cliDocs    bool
	configDocs bool
)

// docsCmd represents the docs command.
var docsCmd = &cobra.Command{
	Use:    "docs <output> [flags]",
	Short:  "Generate documentation for the CLI or config",
	Long:   ``,
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := args[0]

		if err := os.MkdirAll(out, 0o777); err != nil {
			return fmt.Errorf("failed to create output directory %q", out)
		}

		all := !cliDocs && !configDocs

		if cliDocs || all {
			if err := doc.GenMarkdownTreeCustom(rootCmd, out, filePrepender, linkHandler); err != nil {
				return fmt.Errorf("failed to generate docs: %w", err)
			}
		}

		if configDocs || all {
			if err := v1alpha1.GetDoc().Write(out); err != nil {
				return fmt.Errorf("failed to generate docs: %w", err)
			}
		}

		return nil
	},
}

func init() {
	docsCmd.Flags().BoolVarP(&configDocs, "config", "c", false, "generate docs for v1alpha1 configs")
	docsCmd.Flags().BoolVarP(&cliDocs, "cli", "C", false, "generate docs for CLI commands")
	rootCmd.AddCommand(docsCmd)
}
