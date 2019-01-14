/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"fmt"
	"os"

	"github.com/autonomy/talos/internal/app/osctl/internal/client"
	"github.com/autonomy/talos/internal/pkg/constants"
	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/spf13/cobra"
)

// psCmd represents the processes command
var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List processes",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := client.NewDefaultClientCredentials()
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
		if err := c.Processes(namespace); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	psCmd.Flags().BoolVarP(&kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")
	rootCmd.AddCommand(psCmd)
}
