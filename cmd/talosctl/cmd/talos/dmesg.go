// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

var dmesgTail bool

// DmesgCmd represents the dmesg command.
var DmesgCmd = &cobra.Command{
	Use:   "dmesg",
	Short: "Retrieve kernel logs",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			stream, err := c.Dmesg(ctx, follow, dmesgTail)
			if err != nil {
				return fmt.Errorf("error getting dmesg: %w", err)
			}

			return helpers.ReadGRPCStream(stream, func(data *common.Data, node string, multipleNodes bool) error {
				if data.Bytes != nil {
					fmt.Printf("%s: %s", node, data.Bytes)
				}

				return nil
			})
		})
	},
}

func init() {
	addCommand(DmesgCmd)
	DmesgCmd.Flags().BoolVarP(&follow, "follow", "f", false, "specify if the kernel log should be streamed")
	DmesgCmd.Flags().BoolVarP(&dmesgTail, "tail", "", false, "specify if only new messages should be sent (makes sense only when combined with --follow)")
}
