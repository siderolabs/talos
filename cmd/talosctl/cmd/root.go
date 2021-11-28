// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/talosctl/cmd/mgmt"
	"github.com/talos-systems/talos/cmd/talosctl/cmd/talos"
	"github.com/talos-systems/talos/pkg/cli"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
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

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	defaultTalosConfig, err := clientconfig.GetDefaultPath()
	if err != nil {
		return err
	}

	rootCmd.PersistentFlags().StringVar(&talos.Talosconfig, "talosconfig", defaultTalosConfig, "The path to the Talos configuration file")
	rootCmd.PersistentFlags().StringVar(&talos.Cmdcontext, "context", "", "Context to be used in command")
	rootCmd.PersistentFlags().StringSliceVarP(&talos.Nodes, "nodes", "n", []string{}, "target the specified nodes")
	rootCmd.PersistentFlags().StringSliceVarP(&talos.Endpoints, "endpoints", "e", []string{}, "override default endpoints in Talos configuration")
	cli.Should(rootCmd.RegisterFlagCompletionFunc("context", talos.CompleteConfigContext))
	cli.Should(rootCmd.RegisterFlagCompletionFunc("nodes", talos.CompleteNodes))

	cmd, err := rootCmd.ExecuteC()
	if err != nil {
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

func init() {
	for _, cmd := range append(talos.Commands, mgmt.Commands...) {
		rootCmd.AddCommand(cmd)
	}
}
