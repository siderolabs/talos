// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

// rollbackCmd represents the rollback command.
var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback a node to the previous installation",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClientAndNodes(cmd.Context(), func(ctx context.Context, c *client.Client, nodes []string) error {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			responseChan := multiplex.Unary(
				ctx, nodes,
				func(ctx context.Context) (struct{}, error) {
					return struct{}{}, c.Rollback(ctx)
				},
			)

			var errs error

			for resp := range responseChan {
				if resp.Err != nil {
					errs = errors.Join(errs, fmt.Errorf("error executing rollback on node %s: %w", resp.Node, resp.Err))
				}
			}

			return errs
		})
	},
}

func init() {
	addCommand(rollbackCmd)
}
