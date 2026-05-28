// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/machinery/formatters"
)

// mountsCmd represents the mounts command.
var mountsCmd = &cobra.Command{
	Use:     "mounts",
	Aliases: []string{"mount"},
	Short:   "List mounts",
	Long:    ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClientAndNodes(cmd.Context(), func(ctx context.Context, c *client.Client, nodes []string) error {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			responseChan := multiplex.Unary(
				ctx, nodes,
				func(ctx context.Context) (*machineapi.MountsResponse, error) {
					return c.Mounts(ctx)
				},
			)

			return formatters.RenderMountsStream(os.Stdout, responseChan)
		})
	},
}

func init() {
	addCommand(mountsCmd)
}
