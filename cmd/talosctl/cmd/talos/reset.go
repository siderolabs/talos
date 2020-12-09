// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var resetCmdFlags struct {
	graceful           bool
	reboot             bool
	systemLabelsToWipe []string
}

// resetCmd represents the reset command.
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset a node",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			var systemPartitionsToWipe []*machine.ResetPartitionSpec

			for _, label := range resetCmdFlags.systemLabelsToWipe {
				systemPartitionsToWipe = append(systemPartitionsToWipe, &machine.ResetPartitionSpec{
					Label: label,
					Wipe:  true,
				})
			}

			if err := c.ResetGeneric(ctx, &machine.ResetRequest{
				Graceful:               resetCmdFlags.graceful,
				Reboot:                 resetCmdFlags.reboot,
				SystemPartitionsToWipe: systemPartitionsToWipe,
			}); err != nil {
				return fmt.Errorf("error executing reset: %s", err)
			}

			return nil
		})
	},
}

func init() {
	resetCmd.Flags().BoolVar(&resetCmdFlags.graceful, "graceful", true, "if true, attempt to cordon/drain node and leave etcd (if applicable)")
	resetCmd.Flags().BoolVar(&resetCmdFlags.reboot, "reboot", false, "if true, reboot the node after resetting instead of shutting down")
	resetCmd.Flags().StringSliceVar(&resetCmdFlags.systemLabelsToWipe, "system-labels-to-wipe", nil, "if set, just wipe selected system disk partitions by label but keep other partitions intact")
	addCommand(resetCmd)
}
