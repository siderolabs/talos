// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

var rebootCmdFlags struct {
	nodes []string
}

// rebootCmd represents the cluster reboot command.
var rebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Forcefully reboots cluster nodes",
	Long: `Forcefully reboots cluster nodes by restarting the underlying VMs.

By default all nodes are rebooted; pass --node (repeatable) to reboot only specific
nodes, matched by name or IP address. Local and remote QEMU clusters are supported.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return reboot(cmd.Context())
	},
}

func reboot(ctx context.Context) error {
	provisioner, cluster, err := rebootProvisionerAndCluster(ctx)
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint:errcheck

	rebooter, ok := provisioner.(provision.RebootProvisioner)
	if !ok {
		return fmt.Errorf("provisioner %q does not support rebooting nodes", cluster.Provisioner())
	}

	nodes, err := selectRebootNodes(cluster.Info().Nodes, rebootCmdFlags.nodes)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		fmt.Printf("rebooting node %q\n", node.Name)

		if err := rebooter.RebootNode(ctx, cluster, node); err != nil {
			return err
		}
	}

	return nil
}

func rebootProvisionerAndCluster(ctx context.Context) (provision.Provisioner, provision.Cluster, error) {
	if PersistentFlags.RemoteEndpoint != "" {
		provisioner, err := providers.Factory(ctx, providers.RemoteProviderName, providers.WithRemoteEndpoint(PersistentFlags.RemoteEndpoint))
		if err != nil {
			return nil, nil, err
		}

		cluster, err := provisioner.Reflect(ctx, PersistentFlags.ClusterName, "")
		if err != nil {
			provisioner.Close() //nolint:errcheck

			return nil, nil, err
		}

		return provisioner, cluster, nil
	}

	state, err := provision.ReadState(ctx, PersistentFlags.ClusterName, PersistentFlags.StateDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read cluster state: %w", err)
	}

	provisioner, err := providers.Factory(ctx, state.ProvisionerName)
	if err != nil {
		return nil, nil, err
	}

	cluster, err := provisioner.Reflect(ctx, PersistentFlags.ClusterName, PersistentFlags.StateDir)
	if err != nil {
		provisioner.Close() //nolint:errcheck

		return nil, nil, err
	}

	return provisioner, cluster, nil
}

// selectRebootNodes returns the nodes matching the given filters (by name or IP).
// An empty filter list selects all nodes.
func selectRebootNodes(all []provision.NodeInfo, filters []string) ([]provision.NodeInfo, error) {
	if len(filters) == 0 {
		return all, nil
	}

	selected := make([]provision.NodeInfo, 0, len(filters))

	for _, filter := range filters {
		idx := slices.IndexFunc(all, func(node provision.NodeInfo) bool {
			return node.Name == filter || slices.ContainsFunc(node.IPs, func(ip netip.Addr) bool {
				return ip.String() == filter
			})
		})

		if idx < 0 {
			return nil, fmt.Errorf("no node found matching %q", filter)
		}

		selected = append(selected, all[idx])
	}

	return selected, nil
}

func init() {
	rebootCmd.Flags().StringSliceVarP(&rebootCmdFlags.nodes, "node", "n", nil, "node name or IP to reboot, can be repeated (default: all nodes)")

	Cmd.AddCommand(rebootCmd)
}
