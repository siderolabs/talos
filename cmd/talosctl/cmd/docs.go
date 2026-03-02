// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cmd provides the talosctl command implementation.
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
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/hardware"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime/extensions"
	"github.com/siderolabs/talos/pkg/machinery/config/types/security"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	v1alpha1 "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

func frontmatter(title, description string) string {
	var buf bytes.Buffer

	buf.WriteString("---\n")

	if err := yaml.NewEncoder(&buf).Encode(map[string]string{
		"title":       title,
		"description": description,
	}); err != nil {
		panic(err)
	}

	buf.WriteString("---\n")
	buf.WriteString("\n")
	buf.WriteString("<!-- markdownlint-disable -->\n\n")

	return buf.String()
}

func linkHandler(name string) string {
	base := strings.TrimSuffix(name, path.Ext(name))

	base = strings.ReplaceAll(base, "_", "-")

	return "#" + strings.ToLower(base)
}

const cliDescription = "Talosctl CLI tool reference."

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
			for _, pkg := range []struct {
				name    string
				fileDoc *encoder.FileDoc
			}{
				{
					name:    "network",
					fileDoc: network.GetFileDoc(),
				},
				{
					name:    "runtime",
					fileDoc: runtime.GetFileDoc(),
				},
				{
					name:    "siderolink",
					fileDoc: siderolink.GetFileDoc(),
				},
				{
					name:    "v1alpha1",
					fileDoc: v1alpha1.GetFileDoc(),
				},
				{
					name:    "extensions",
					fileDoc: extensions.GetFileDoc(),
				},
				{
					name:    "security",
					fileDoc: security.GetFileDoc(),
				},
				{
					name:    "block",
					fileDoc: block.GetFileDoc(),
				},
				{
					name:    "hardware",
					fileDoc: hardware.GetFileDoc(),
				},
				{
					name:    "cri",
					fileDoc: cri.GetFileDoc(),
				},
				{
					name:    "k8s",
					fileDoc: k8s.GetFileDoc(),
				},
			} {
				path := filepath.Join(dir, pkg.name)

				if err := os.MkdirAll(path, 0o777); err != nil {
					return fmt.Errorf("failed to create output directory %q", path)
				}

				if err := pkg.fileDoc.Write(path, frontmatter); err != nil {
					return fmt.Errorf("failed to generate docs: %w", err)
				}
			}
		}

		return nil
	},
}

// GenMarkdownReference is the same as GenMarkdownTree, but
// with custom filePrepender and linkHandler.
//
//nolint:gocyclo
func GenMarkdownReference(cmd *cobra.Command, w io.Writer, linkHandler func(string) string) error {
	for _, c := range cmd.Commands() {
		// Generate docs for children of the cluster create command although the command itself is hidden.
		if cmd.Name() == "cluster" && c.Name() == "create" {
			if err := GenMarkdownReference(c, w, linkHandler); err != nil {
				return err
			}
		}

		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}

		if err := GenMarkdownReference(c, w, linkHandler); err != nil {
			return err
		}
	}

	// Skip generating docs for the cluster create command itself and only generate docs for children.
	// TODO: remove once "cluster create" is completely migrated to "cluster create dev".
	if cmd.Name() == "create" && cmd.Parent() != nil && cmd.Parent().Name() == "cluster" {
		return nil
	}

	return doc.GenMarkdownCustom(cmd, w, linkHandler)
}

func init() {
	docsCmd.Flags().BoolVar(&configDocs, "config", false, "generate documentation for the default configuration schema")
	docsCmd.Flags().BoolVar(&cliDocs, "cli", false, "generate documentation for the CLI")
	rootCmd.AddCommand(docsCmd)
}
