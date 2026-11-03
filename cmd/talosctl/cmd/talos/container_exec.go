// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

package talos

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

var containerExecCmdFlags struct {
	stdin bool
	tty   bool
}

// containerExecCmd represents the container exec command.
var containerExecCmd = &cobra.Command{
	Use:   "exec <container> -- <command> [<args>...]",
	Short: "Run a command in a running container",
	Long: `Runs a command in a running container declared with a ContainerConfig document, as the container's
own process runs: in its namespaces and cgroup, as its user, with its capabilities.

The command runs as given, no shell is assumed. talosctl exits with the exit code of the command.
stdin is passed to the command with --stdin, and a terminal is allocated with --tty; without a
terminal, stdout and stderr of the command are kept apart. Interrupting talosctl ends the command;
processes it started in the background are not tracked.

The command and its arguments are logged on the node; pass secrets on stdin, which is not.`,
	Example: `  # Run a command
    talosctl container exec app -- /usr/bin/status

  # Run a command and pass data on its stdin
    talosctl container exec -i app -- /usr/bin/import < data.json

  # Open a shell in a container
    talosctl container exec -it nginx -- /bin/sh`,
	Args: cobra.MinimumNArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			// The command runs on the node, local files are of no use.
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return completeResourceID(cmd.Context(), containers.ContainerStatusType, containers.NamespaceName)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		ctx, c, _, err := clientFactory.BuildClientEnforceSingleNode(ctx, "container exec")
		if err != nil {
			return err
		}

		if containerExecCmdFlags.stdin && containerExecCmdFlags.tty && !term.IsTerminal(int(os.Stdin.Fd())) {
			return errors.New("--tty with --stdin requires stdin to be a terminal")
		}

		opts, restoreTerminal, err := containerStreamTerminal(ctx, containerExecCmdFlags.stdin, containerExecCmdFlags.tty)
		if err != nil {
			return err
		}

		result, err := c.ContainerExec(ctx, &machine.DebugContainerExecRequestSpec{
			Container: args[0],
			Args:      args[1:],
			Tty:       containerExecCmdFlags.tty,
		}, opts)

		restoreTerminal()

		if err != nil {
			return err
		}

		if result.ExitCode != 0 {
			return &common.ExitError{Code: result.ExitCode}
		}

		return nil
	},
}

func init() {
	containerExecCmd.Flags().BoolVarP(&containerExecCmdFlags.stdin, "stdin", "i", false, "pass stdin to the command")
	containerExecCmd.Flags().BoolVarP(&containerExecCmdFlags.tty, "tty", "t", false, "allocate a terminal for the command; with --stdin, stdin has to be a terminal")

	containerCmd.AddCommand(containerExecCmd)
}
