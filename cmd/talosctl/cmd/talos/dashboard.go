// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/talosctl/cmd/talos/dashboard"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var dashboardCmdFlags struct {
	interval time.Duration
}

// dashboardCmd represents the monitor command.
var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Cluster dashboard with real-time metrics",
	Long: `Provide quick UI to navigate through node real-time metrics.

Keyboard shortcuts:

 - h, <Left>: switch one node to the left
 - l, <Right>: switch one node to the right
 - j, <Down>: scroll process list down
 - k, <Up>: scroll process list up
 - <C-d>: scroll process list half page down
 - <C-u>: scroll process list half page up
 - <C-f>: scroll process list one page down
 - <C-b>: scroll process list one page up
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			return dashboard.Main(ctx, c, dashboardCmdFlags.interval)
		})
	},
}

func init() {
	dashboardCmd.Flags().DurationVarP(&dashboardCmdFlags.interval, "update-interval", "d", 3*time.Second, "interval between updates")
	addCommand(dashboardCmd)
}
