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
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var resetCmdFlags struct {
	graceful           bool
	reboot             bool
	systemLabelsToWipe []string
	wait               bool
	debug              bool
}

// resetCmd represents the reset command.
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset a node",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if resetCmdFlags.debug {
			resetCmdFlags.wait = true
		}

		resetRequest := buildResetRequest()

		if !resetCmdFlags.wait {
			return WithClient(func(ctx context.Context, c *client.Client) error {
				if err := helpers.ClientVersionCheck(ctx, c); err != nil {
					return err
				}

				if err := c.ResetGeneric(ctx, resetRequest); err != nil {
					return fmt.Errorf("error executing reset: %s", err)
				}

				return nil
			})
		}

		actionFn := func(ctx context.Context, c *client.Client) (string, error) {
			return resetGetActorID(ctx, c, resetRequest)
		}

		var postCheckFn func(context.Context, *client.Client) error

		if resetCmdFlags.reboot {
			postCheckFn = func(ctx context.Context, c *client.Client) error {
				return WithClientMaintenance(nil,
					func(ctx context.Context, cli *client.Client) error {
						_, err := cli.Disks(ctx)

						return err
					})
			}
		}

		cmd.SilenceErrors = true

		return action.NewTracker(&GlobalArgs, action.StopAllServicesEventFn, actionFn, postCheckFn, resetCmdFlags.debug).Run()
	},
}

func buildResetRequest() *machineapi.ResetRequest {
	systemPartitionsToWipe := make([]*machineapi.ResetPartitionSpec, 0, len(resetCmdFlags.systemLabelsToWipe))

	for _, label := range resetCmdFlags.systemLabelsToWipe {
		systemPartitionsToWipe = append(systemPartitionsToWipe, &machineapi.ResetPartitionSpec{
			Label: label,
			Wipe:  true,
		})
	}

	return &machineapi.ResetRequest{
		Graceful:               resetCmdFlags.graceful,
		Reboot:                 resetCmdFlags.reboot,
		SystemPartitionsToWipe: systemPartitionsToWipe,
	}
}

func resetGetActorID(ctx context.Context, c *client.Client, req *machineapi.ResetRequest) (string, error) {
	resp, err := c.ResetGenericWithResponse(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.GetMessages()) == 0 {
		return "", fmt.Errorf("no messages returned from action run")
	}

	return resp.GetMessages()[0].GetActorId(), nil
}

func init() {
	resetCmd.Flags().BoolVar(&resetCmdFlags.graceful, "graceful", true, "if true, attempt to cordon/drain node and leave etcd (if applicable)")
	resetCmd.Flags().BoolVar(&resetCmdFlags.reboot, "reboot", false, "if true, reboot the node after resetting instead of shutting down")
	resetCmd.Flags().StringSliceVar(&resetCmdFlags.systemLabelsToWipe, "system-labels-to-wipe", nil, "if set, just wipe selected system disk partitions by label but keep other partitions intact")
	resetCmd.Flags().BoolVar(&resetCmdFlags.wait, "wait", false, "wait for the operation to complete, tracking its progress. always set to true when --debug is set")
	resetCmd.Flags().BoolVar(&resetCmdFlags.debug, "debug", false, "debug operation from kernel logs. --no-wait is set to false when this flag is set")
	addCommand(resetCmd)
}
