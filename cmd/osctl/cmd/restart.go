// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: dupl,golint
package cmd

import (
	"os"

	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/spf13/cobra"

	osapi "github.com/talos-systems/talos/api/os"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/pkg/constants"
)

// restartCmd represents the restart command
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart a process",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			if err := cmd.Usage(); err != nil {
				os.Exit(1)
			}
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			var namespace string
			if kubernetes {
				namespace = criconstants.K8sContainerdNamespace
			} else {
				namespace = constants.SystemContainerdNamespace
			}
			driver := osapi.ContainerDriver_CONTAINERD
			if useCRI {
				driver = osapi.ContainerDriver_CRI
			}
			if err := c.Restart(globalCtx, namespace, driver, args[0]); err != nil {
				helpers.Fatalf("error restarting process: %s", err)
			}
		})
	},
}

func init() {
	restartCmd.Flags().BoolVarP(&kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")
	restartCmd.Flags().BoolVarP(&useCRI, "use-cri", "c", false, "use the CRI driver")
	rootCmd.AddCommand(restartCmd)
}
