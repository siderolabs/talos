// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/common"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt"
	_ "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create" // import to get the command registered via the init() function.
	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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
	rootCmd.PersistentFlags().StringVar(
		&talos.GlobalArgs.Talosconfig,
		"talosconfig",
		"",
		fmt.Sprintf("The path to the Talos configuration file. Defaults to '%s' env variable if set, otherwise '%s' and '%s' in order.",
			constants.TalosConfigEnvVar,
			filepath.Join("$HOME", constants.TalosDir, constants.TalosconfigFilename),
			filepath.Join(constants.ServiceAccountMountPath, constants.TalosconfigFilename),
		),
	)
	rootCmd.PersistentFlags().StringVar(&talos.GlobalArgs.CmdContext, "context", "", "Context to be used in command")
	rootCmd.PersistentFlags().StringSliceVarP(&talos.GlobalArgs.Nodes, "nodes", "n", []string{}, "target the specified nodes")
	rootCmd.PersistentFlags().StringSliceVarP(&talos.GlobalArgs.Endpoints, "endpoints", "e", []string{}, "override default endpoints in Talos configuration")
	cli.Should(rootCmd.RegisterFlagCompletionFunc("context", talos.CompleteConfigContext))
	cli.Should(rootCmd.RegisterFlagCompletionFunc("nodes", talos.CompleteNodes))
	rootCmd.PersistentFlags().StringVar(&talos.GlobalArgs.Cluster, "cluster", "", "Cluster to connect to if a proxy endpoint is used.")

	cmd, err := rootCmd.ExecuteContextC(context.Background())
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

func init() {
	for _, cmd := range slices.Concat(talos.Commands, mgmt.Commands) {
		rootCmd.AddCommand(cmd)
	}
}
