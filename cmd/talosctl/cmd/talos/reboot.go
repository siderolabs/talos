// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/action"
	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var rebootCmdFlags struct {
	mode  string
	wait  bool
	debug bool
}

// rebootCmd represents the reboot command.
var rebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot a node",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rebootCmdFlags.debug {
			rebootCmdFlags.wait = true
		}

		var opts []client.RebootMode

		switch rebootCmdFlags.mode {
		// skips kexec and reboots with power cycle
		case "powercycle":
			opts = append(opts, client.WithPowerCycle)
		case "default":
		default:
			return fmt.Errorf("invalid reboot mode: %q", rebootCmdFlags.mode)
		}

		if !rebootCmdFlags.wait {
			return WithClient(func(ctx context.Context, c *client.Client) error {
				if err := helpers.ClientVersionCheck(ctx, c); err != nil {
					return err
				}

				if err := c.Reboot(ctx, opts...); err != nil {
					return fmt.Errorf("error executing reboot: %s", err)
				}

				return nil
			})
		}

		cmd.SilenceErrors = true

		postCheckFn := func(ctx context.Context, c *client.Client) error {
			_, err := c.Disks(ctx)

			return err
		}

		return action.NewTracker(&GlobalArgs, action.MachineReadyEventFn, rebootGetActorID, postCheckFn, rebootCmdFlags.debug).Run()
	},
}

func rebootGetActorID(ctx context.Context, c *client.Client) (string, error) {
	resp, err := c.RebootWithResponse(ctx)
	if err != nil {
		return "", err
	}

	if len(resp.GetMessages()) == 0 {
		return "", fmt.Errorf("no messages returned from action run")
	}

	return resp.GetMessages()[0].GetActorId(), nil
}

func init() {
	rebootCmd.Flags().StringVarP(&rebootCmdFlags.mode, "mode", "m", "default", "select the reboot mode: \"default\", \"powercycle\" (skips kexec)")
	rebootCmd.Flags().BoolVar(&rebootCmdFlags.wait, "wait", false, "wait for the operation to complete, tracking its progress. always set to true when --debug is set")
	rebootCmd.Flags().BoolVar(&rebootCmdFlags.debug, "debug", false, "debug operation from kernel logs. --no-wait is set to false when this flag is set")
	addCommand(rebootCmd)
}
