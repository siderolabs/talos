// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/internal/pkg/dashboard"
)

var dashboardCmdFlags struct {
	interval time.Duration
}

// dashboardCmd represents the monitor command.
var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Cluster dashboard with node overview, containers, logs and real-time metrics",
	Long: `Provide a text-based UI to navigate node overview, containers, logs and real-time metrics.

Keyboard shortcuts:

 - h, <Left> - switch one node to the left
 - l, <Right> - switch one node to the right
 - j, <Down> - scroll logs/process list down
 - k, <Up> - scroll logs/process list up
 - <C-d> - scroll logs/process list half page down
 - <C-u> - scroll logs/process list half page up
 - <C-f> - scroll logs/process list one page down
 - <C-b> - scroll logs/process list one page up
 - <C-z> - pause updates

Containers screen:

 - <Enter> - open the selected container's details and logs
 - <Esc> - go back one level
 - / - filter the container list, or the logs when viewing them
 - s - cycle the list ordering: name, health, restarts, cpu, memory
 - n - cycle the containerd namespace: taloscontainers, system, k8s.io
 - y - view the selected container's resources as YAML; <Tab> cycles between them
 - i - toggle the instance history of the selected container
 - f - freeze or resume log following
 - w - toggle log line wrapping
 - <Tab> - move the focus between the details and the logs
 - g, G - jump to the top or the bottom
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		c, err := clientFactory.BuildRandomEndpointClient(ctx)
		if err != nil {
			return err
		}

		return dashboard.Run(
			ctx, c,
			dashboard.WithInterval(dashboardCmdFlags.interval),
			dashboard.WithScreens(dashboard.ScreenSummary, dashboard.ScreenMonitor, dashboard.ScreenContainers, dashboard.ScreenResourceExplorer),
			dashboard.WithAllowExitKeys(true),
			dashboard.WithNodes(clientFactory.Nodes()...),
		)
	},
}

func init() {
	dashboardCmd.Flags().DurationVarP(&dashboardCmdFlags.interval, "update-interval", "d", 3*time.Second, "interval between updates")
	addCommand(dashboardCmd)
}
