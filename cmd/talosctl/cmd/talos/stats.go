// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/api/common"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// statsCmd represents the stats command.
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Get container stats",
	Long:  ``,
	Args:  cobra.NoArgs,
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

			var remotePeer peer.Peer

			resp, err := c.Stats(ctx, namespace, driver, grpc.Peer(&remotePeer))
			if err != nil {
				if resp == nil {
					return fmt.Errorf("error getting stats: %s", err)
				}

				cli.Warning("%s", err)
			}

			return statsRender(&remotePeer, resp)
		})
	},
}

func statsRender(remotePeer *peer.Peer, resp *machineapi.StatsResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	fmt.Fprintln(w, "NODE\tNAMESPACE\tID\tMEMORY(MB)\tCPU")

	defaultNode := client.AddrFromPeer(remotePeer)

	for _, msg := range resp.Messages {
		resp := msg
		sort.Slice(resp.Stats,
			func(i, j int) bool {
				return strings.Compare(resp.Stats[i].Id, resp.Stats[j].Id) < 0
			})

		for _, s := range resp.Stats {
			display := s.Id
			if s.Id != s.PodId {
				// container in a sandbox
				display = "└─ " + display
			}

			node := defaultNode

			if resp.Metadata != nil {
				node = resp.Metadata.Hostname
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%.2f\t%d\n", node, s.Namespace, display, float64(s.MemoryUsage)*1e-6, s.CpuUsage)
		}
	}

	return w.Flush()
}

func init() {
	statsCmd.Flags().BoolVarP(&kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")

	statsCmd.Flags().BoolP("use-cri", "c", false, "use the CRI driver")
	statsCmd.Flags().MarkHidden("use-cri") //nolint:errcheck

	addCommand(statsCmd)
}
