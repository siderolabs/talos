/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/internal/app/osd/proto"
	"github.com/talos-systems/talos/internal/pkg/constants"
)

// psCmd represents the processes command
var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List processes",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			var namespace string
			if kubernetes {
				namespace = criconstants.K8sContainerdNamespace
			} else {
				namespace = constants.SystemContainerdNamespace
			}
			reply, err := c.Processes(context.TODO(), namespace)

			if err != nil {
				helpers.Fatalf("error getting process list: %s", err)
			}

			processesRender(reply)
		})
	},
}

func processesRender(reply *proto.ProcessesReply) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tID\tIMAGE\tPID\tSTATUS")
	for _, p := range reply.Processes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", p.Namespace, p.Id, p.Image, p.Pid, p.Status)
	}
	helpers.Should(w.Flush())
}

func init() {
	psCmd.Flags().BoolVarP(&kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")
	psCmd.Flags().StringVarP(&target, "target", "t", "", "target the specificed node")
	rootCmd.AddCommand(psCmd)
}
