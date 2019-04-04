/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"fmt"
	"os"

	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/internal/app/osd/proto"
	"github.com/talos-systems/talos/internal/pkg/constants"
)

var (
	timeout int32
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
		creds, err := client.NewDefaultClientCredentials(talosconfig)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		c, err := client.NewClient(constants.OsdPort, creds)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		var namespace string
		if kubernetes {
			namespace = criconstants.K8sContainerdNamespace
		} else {
			namespace = constants.SystemContainerdNamespace
		}
		r := &proto.RestartRequest{
			Id:        args[0],
			Namespace: namespace,
			Timeout:   timeout,
		}
		if err := c.Restart(r); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	restartCmd.Flags().Int32VarP(&timeout, "timeout", "t", 60, "the timeout duration in seconds")
	restartCmd.Flags().BoolVarP(&kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")
	rootCmd.AddCommand(restartCmd)
}
