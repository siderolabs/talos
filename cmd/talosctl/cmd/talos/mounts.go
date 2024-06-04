// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/formatters"
)

// MountsCmd represents the mounts command.
var MountsCmd = &cobra.Command{
	Use:     "mounts",
	Aliases: []string{"mount"},
	Short:   "List mounts",
	Long:    ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			var remotePeer peer.Peer

			resp, err := c.Mounts(ctx, grpc.Peer(&remotePeer))
			if err != nil {
				if resp == nil {
					return fmt.Errorf("error getting mount information: %s", err)
				}

				cli.Warning("%s", err)
			}

			return formatters.RenderMounts(resp, os.Stdout, &remotePeer)
		})
	},
}

func init() {
	addCommand(MountsCmd)
}
