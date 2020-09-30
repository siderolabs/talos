// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/provision"
	"github.com/talos-systems/talos/pkg/provision/providers"
)

// showCmd represents the cluster show command.
var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Shows info about a local provisioned kubernetes cluster",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.WithContext(context.Background(), show)
	},
}

func show(ctx context.Context) error {
	provisioner, err := providers.Factory(ctx, provisionerName)
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint: errcheck

	cluster, err := provisioner.Reflect(ctx, clusterName, stateDir)
	if err != nil {
		return err
	}

	return showCluster(cluster)
}

func showCluster(cluster provision.Cluster) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "PROVISIONER\t%s\n", cluster.Provisioner())
	fmt.Fprintf(w, "NAME\t%s\n", cluster.Info().ClusterName)
	fmt.Fprintf(w, "NETWORK NAME\t%s\n", cluster.Info().Network.Name)

	ones, _ := cluster.Info().Network.CIDR.Mask.Size()
	fmt.Fprintf(w, "NETWORK CIDR\t%s/%d\n", cluster.Info().Network.CIDR.IP, ones)
	fmt.Fprintf(w, "NETWORK GATEWAY\t%s\n", cluster.Info().Network.GatewayAddr)
	fmt.Fprintf(w, "NETWORK MTU\t%d\n", cluster.Info().Network.MTU)

	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Fprint(os.Stdout, "\nNODES:\n\n")

	w = tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	fmt.Fprintf(w, "NAME\tTYPE\tIP\tCPU\tRAM\tDISK\n")

	nodes := cluster.Info().Nodes
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })

	for _, node := range nodes {
		cpus := "-"
		if node.NanoCPUs > 0 {
			cpus = fmt.Sprintf("%.2f", float64(node.NanoCPUs)/1000.0/1000.0/1000.0)
		}

		mem := "-"
		if node.Memory > 0 {
			mem = humanize.Bytes(uint64(node.Memory))
		}

		disk := "-"
		if node.DiskSize > 0 {
			disk = humanize.Bytes(uint64(node.DiskSize))
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			node.Name,
			node.Type,
			node.PrivateIP,
			cpus,
			mem,
			disk,
		)
	}

	return w.Flush()
}

func init() {
	Cmd.AddCommand(showCmd)
}
