/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
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

// statsCmd represents the processes command
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Get processes stats",
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
			reply, err := c.Stats(globalCtx, namespace)
			if reply == nil && err != nil {
				// fatal error
				helpers.Fatalf("error getting stats: %s", err)
			}

			statsRender(reply)

			if err != nil {
				// some errors encountered, but not fatal
				fmt.Fprintf(os.Stderr, "errors while getting stats: %s", err)
			}
		})
	},
}

func statsRender(reply *proto.StatsReply) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tID\tMEMORY(MB)\tCPU")
	for _, s := range reply.Stats {
		fmt.Fprintf(w, "%s\t%s\t%.2f\t%d\n", s.Namespace, s.Id, float64(s.MemoryUsage)*1e-6, s.CpuUsage)
	}
	helpers.Should(w.Flush())
}

func init() {
	statsCmd.Flags().BoolVarP(&kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")
	statsCmd.Flags().StringVarP(&target, "target", "t", "", "target the specificed node")
	rootCmd.AddCommand(statsCmd)
}
