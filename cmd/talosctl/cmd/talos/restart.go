// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// restartCmd represents the restart command.
var restartCmd = &cobra.Command{
	Use:   "restart <id>",
	Short: "Restart a process",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			var (
				namespace string
				driver    common.ContainerDriver
			)

			if kubernetes {
				namespace = criconstants.K8sContainerdNamespace
				driver = common.ContainerDriver_CRI
			} else {
				namespace = constants.SystemContainerdNamespace
				driver = common.ContainerDriver_CONTAINERD
			}

			if err := c.Restart(ctx, namespace, driver, args[0]); err != nil {
				return fmt.Errorf("error restarting process: %s", err)
			}

			return nil
		})
	},
}

func init() {
	restartCmd.Flags().BoolVarP(&kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")

	restartCmd.Flags().BoolP("use-cri", "c", false, "use the CRI driver")
	restartCmd.Flags().MarkHidden("use-cri") //nolint:errcheck

	addCommand(restartCmd)
}
