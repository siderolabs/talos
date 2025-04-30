// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"cmp"
	"context"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/dustin/go-humanize"
	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers"
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
	provisioner, err := providers.Factory(ctx, Flags.ProvisionerName)
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint:errcheck

	cluster, err := provisioner.Reflect(ctx, Flags.ClusterName, Flags.StateDir)
	if err != nil {
		return err
	}

	return ShowCluster(cluster)
}

// ShowCluster prints the details about the cluster to the terminal.
func ShowCluster(cluster provision.Cluster) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "PROVISIONER\t%s\n", cluster.Provisioner())
	fmt.Fprintf(w, "NAME\t%s\n", cluster.Info().ClusterName)
	fmt.Fprintf(w, "NETWORK NAME\t%s\n", cluster.Info().Network.Name)

	cidrs := xslices.Map(cluster.Info().Network.CIDRs, netip.Prefix.String)

	fmt.Fprintf(w, "NETWORK CIDR\t%s\n", strings.Join(cidrs, ","))

	gateways := xslices.Map(cluster.Info().Network.GatewayAddrs, netip.Addr.String)

	fmt.Fprintf(w, "NETWORK GATEWAY\t%s\n", strings.Join(gateways, ","))
	fmt.Fprintf(w, "NETWORK MTU\t%d\n", cluster.Info().Network.MTU)
	fmt.Fprintf(w, "KUBERNETES ENDPOINT\t%s\n", cluster.Info().KubernetesEndpoint)

	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Fprint(os.Stdout, "\nNODES:\n\n")

	w = tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	fmt.Fprintf(w, "NAME\tTYPE\tIP\tCPU\tRAM\tDISK\n")

	nodes := cluster.Info().Nodes
	slices.SortFunc(nodes, func(a, b provision.NodeInfo) int { return cmp.Compare(a.Name, b.Name) })

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
			disk = humanize.Bytes(node.DiskSize)
		}

		ips := xslices.Map(node.IPs, netip.Addr.String)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			node.Name,
			node.Type,
			strings.Join(ips, ","),
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
