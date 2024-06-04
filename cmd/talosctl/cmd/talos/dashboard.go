// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/internal/pkg/dashboard"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

var dashboardCmdFlags struct {
	interval time.Duration
}

// DashboardCmd represents the monitor command.
var DashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Cluster dashboard with node overview, logs and real-time metrics",
	Long: `Provide a text-based UI to navigate node overview, logs and real-time metrics.

Keyboard shortcuts:

 - h, &lt;Left&gt; - switch one node to the left
 - l, &lt;Right&gt; - switch one node to the right
 - j, &lt;Down&gt; - scroll logs/process list down
 - k, &lt;Up&gt; - scroll logs/process list up
 - &lt;C-d&gt; - scroll logs/process list half page down
 - &lt;C-u&gt; - scroll logs/process list half page up
 - &lt;C-f&gt; - scroll logs/process list one page down
 - &lt;C-b&gt; - scroll logs/process list one page up
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			return dashboard.Run(ctx, c,
				dashboard.WithInterval(dashboardCmdFlags.interval),
				dashboard.WithScreens(dashboard.ScreenSummary, dashboard.ScreenMonitor),
				dashboard.WithAllowExitKeys(true),
			)
		})
	},
}

func init() {
	DashboardCmd.Flags().DurationVarP(&dashboardCmdFlags.interval, "update-interval", "d", 3*time.Second, "interval between updates")
	addCommand(DashboardCmd)
}
