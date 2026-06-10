// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

var dmesgCmdFlags struct {
	global.InsecureFlags

	follow bool
	tail   bool
}

// dmesgCmd represents the dmesg command.
var dmesgCmd = &cobra.Command{
	Use:   "dmesg",
	Short: "Retrieve kernel logs",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &dmesgCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.StreamingViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (machineapi.MachineService_DmesgClient, error) {
				return c.Dmesg(ctx, dmesgCmdFlags.follow, dmesgCmdFlags.tail)
			},
		)

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

				continue
			}

			if resp.Payload.Bytes != nil {
				fmt.Printf("%s: %s", resp.Node, resp.Payload.Bytes)
			}
		}

		return errs
	},
}

func init() {
	addCommand(dmesgCmd)
	dmesgCmd.Flags().BoolVarP(&dmesgCmdFlags.follow, "follow", "f", false, "specify if the kernel log should be streamed")
	dmesgCmd.Flags().BoolVarP(&dmesgCmdFlags.tail, "tail", "", false, "specify if only new messages should be sent (makes sense only when combined with --follow)")
	dmesgCmdFlags.InsecureFlags.AddFlags(dmesgCmd)
}
