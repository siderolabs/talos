// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/machinery/client"
)

// shutdownCmd represents the shutdown command.
var shutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Shutdown a node",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			if err := c.Shutdown(ctx); err != nil {
				return fmt.Errorf("error executing shutdown: %s", err)
			}

			return nil
		})
	},
}

func init() {
	addCommand(shutdownCmd)
}
