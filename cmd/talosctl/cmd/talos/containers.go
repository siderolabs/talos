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

// containersCmd represents the processes command.
var containersCmd = &cobra.Command{
	Use:     "containers",
	Aliases: []string{"c"},
	Short:   "List containers",
	Long:    ``,
	Args:    cobra.NoArgs,
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

			resp, err := c.Containers(ctx, namespace, driver, grpc.Peer(&remotePeer))
			if err != nil {
				if resp == nil {
					return fmt.Errorf("error getting container list: %s", err)
				}

				cli.Warning("%s", err)
			}

			return containerRender(&remotePeer, resp)
		})
	},
}

func containerRender(remotePeer *peer.Peer, resp *machineapi.ContainersResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tNAMESPACE\tID\tIMAGE\tPID\tSTATUS")

	defaultNode := client.AddrFromPeer(remotePeer)

	for _, msg := range resp.Messages {
		resp := msg
		sort.Slice(resp.Containers,
			func(i, j int) bool {
				return strings.Compare(resp.Containers[i].Id, resp.Containers[j].Id) < 0
			})

		for _, p := range resp.Containers {
			display := p.Id
			if p.Id != p.PodId {
				// container in a sandbox
				display = "└─ " + display
			}

			node := defaultNode

			if resp.Metadata != nil {
				node = resp.Metadata.Hostname
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n", node, p.Namespace, display, p.Image, p.Pid, p.Status)
		}
	}

	return w.Flush()
}

func init() {
	containersCmd.Flags().BoolVarP(&kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")

	containersCmd.Flags().BoolP("use-cri", "c", false, "use the CRI driver")
	containersCmd.Flags().MarkHidden("use-cri") //nolint:errcheck

	addCommand(containersCmd)
}
