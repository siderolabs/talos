/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

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
			driver := osapi.ContainerDriver_CONTAINERD
			if useCRI {
				driver = osapi.ContainerDriver_CRI
			}
			reply, err := c.Containers(globalCtx, namespace, driver)
			if err != nil {
				helpers.Fatalf("error getting process list: %s", err)
			}

			containerRender(reply)
		})
	},
}

func containerRender(reply *osapi.ContainersReply) {
	sort.Slice(reply.Containers,
		func(i, j int) bool {
			return strings.Compare(reply.Containers[i].Id, reply.Containers[j].Id) < 0
		})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tID\tIMAGE\tPID\tSTATUS")

	for _, p := range reply.Containers {
		display := p.Id
		if p.Id != p.PodId {
			// container in a sandbox
			display = "└─ " + display
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", p.Namespace, display, p.Image, p.Pid, p.Status)
	}

	helpers.Should(w.Flush())
}

func init() {
	containersCmd.Flags().BoolVarP(&kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")
	containersCmd.Flags().BoolVarP(&useCRI, "use-cri", "c", false, "use the CRI driver")
	rootCmd.AddCommand(containersCmd)
}
