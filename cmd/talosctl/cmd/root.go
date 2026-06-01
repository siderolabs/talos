// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/common"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	_ "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create" // import to get the command registered via the init() function.
	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos"
	"github.com/siderolabs/talos/pkg/cli"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:               "talosctl",
	Short:             "A CLI for out-of-band management of Kubernetes nodes created by Talos",
	Long:              ``,
	SilenceErrors:     true,
	SilenceUsage:      true,
	DisableAutoGenTag: true,
}

// Execute invokes the user-entered command in a cancellable context (with Ctrl^C).
// Handles errors related to incorrect usage and prints the error message to stderr.
func Execute() error {
	cmd, err := cli.WithContextC(context.Background(), rootCmd.ExecuteContextC)
	if err != nil && !common.SuppressErrors {
		fmt.Fprintln(os.Stderr, err.Error())

		errorString := err.Error()
		// TODO: this is a nightmare, but arg-flag related validation returns simple `fmt.Errorf`, no way to distinguish
		//       these errors
		if strings.Contains(errorString, "arg(s)") || strings.Contains(errorString, "flag") || strings.Contains(errorString, "command") {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, cmd.UsageString())
		}
	}

	return err
}

// init adds all child commands to the base command.
func init() {
	const (
		talosGroup   = "talos"
		mgmtGroup    = "mgmt"
		clusterGroup = "cluster"
	)

	rootCmd.AddGroup(&cobra.Group{ID: talosGroup, Title: "Manage running Talos clusters:"})
	rootCmd.AddGroup(&cobra.Group{ID: mgmtGroup, Title: "Commands to generate and manage machine configuration offline:"})
	rootCmd.AddGroup(&cobra.Group{ID: clusterGroup, Title: "Local Talos cluster commands:"})

	for _, cmd := range mgmt.Commands {
		cmd.GroupID = mgmtGroup
		if cmd == cluster.Cmd {
			cmd.GroupID = clusterGroup
		}

		rootCmd.AddCommand(cmd)
	}

	for _, cmd := range talos.Commands {
		cmd.GroupID = talosGroup
		rootCmd.AddCommand(cmd)
	}
}
