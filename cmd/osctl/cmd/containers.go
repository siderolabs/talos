// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: dupl,golint
package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/metadata"

	"github.com/talos-systems/talos/api/common"
	osapi "github.com/talos-systems/talos/api/os"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/pkg/constants"
)

// containersCmd represents the processes command
var containersCmd = &cobra.Command{
	Use:     "containers",
	Aliases: []string{"c"},
	Short:   "List containers",
	Long:    ``,
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
			driver := common.ContainerDriver_CONTAINERD
			if useCRI {
				driver = common.ContainerDriver_CRI
			}

			md := metadata.New(make(map[string]string))
			md.Set("targets", target...)

			reply, err := c.Containers(metadata.NewOutgoingContext(globalCtx, md), namespace, driver)
			if err != nil {
				helpers.Fatalf("error getting process list: %s", err)
			}

			containerRender(reply)
		})
	},
}

func containerRender(reply *osapi.ContainersReply) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tNAMESPACE\tID\tIMAGE\tPID\tSTATUS")

	for _, rep := range reply.Response {
		resp := rep
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

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n", resp.Metadata.Hostname, p.Namespace, display, p.Image, p.Pid, p.Status)
		}
	}

	helpers.Should(w.Flush())
}

func init() {
	containersCmd.Flags().BoolVarP(&kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")
	containersCmd.Flags().BoolVarP(&useCRI, "use-cri", "c", false, "use the CRI driver")
	rootCmd.AddCommand(containersCmd)
}
