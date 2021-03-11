// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	v1alpha1 "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

func frontmatter(title, description string) string {
	frontmatter := "---\n"

	frontmatter += "title: " + title + "\n"
	frontmatter += "desription: " + description + "\n"

	frontmatter += "---\n\n"

	return frontmatter + "<!-- markdownlint-disable -->\n\n"
}

func linkHandler(name string) string {
	base := strings.TrimSuffix(name, path.Ext(name))

	base = strings.ReplaceAll(base, "_", "-")

	return "#" + strings.ToLower(base)
}

const (
	cliDescription           = "Talosctl CLI tool reference."
	configurationDescription = "Talos node configuration file reference."
)

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
		dir := args[0]

		if err := os.MkdirAll(dir, 0o777); err != nil {
			return fmt.Errorf("failed to create output directory %q", dir)
		}

		all := !cliDocs && !configDocs

		if cliDocs || all {
			w := &bytes.Buffer{}

			if err := GenMarkdownReference(rootCmd, w, linkHandler); err != nil {
				return fmt.Errorf("failed to generate docs: %w", err)
			}

			filename := filepath.Join(dir, "cli.md")
			f, err := os.Create(filename)
			if err != nil {
				return err
			}
			//nolint:errcheck
			defer f.Close()

			if _, err := io.WriteString(f, frontmatter("CLI", cliDescription)); err != nil {
				return err
			}

			if _, err := io.WriteString(f, w.String()); err != nil {
				return err
			}
		}

		if configDocs || all {
			if err := v1alpha1.GetConfigurationDoc().Write(dir, frontmatter("Configuration", configurationDescription)); err != nil {
				return fmt.Errorf("failed to generate docs: %w", err)
			}
		}

		return nil
	},
}

// GenMarkdownReference is the the same as GenMarkdownTree, but
// with custom filePrepender and linkHandler.
func GenMarkdownReference(cmd *cobra.Command, w io.Writer, linkHandler func(string) string) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}

		if err := GenMarkdownReference(c, w, linkHandler); err != nil {
			return err
		}
	}

	return doc.GenMarkdownCustom(cmd, w, linkHandler)
}

func init() {
	docsCmd.Flags().BoolVar(&configDocs, "config", false, "generate documentation for the default configuration schema")
	docsCmd.Flags().BoolVar(&cliDocs, "cli", false, "generate documentation for the CLI")
	rootCmd.AddCommand(docsCmd)
}
