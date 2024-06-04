// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

var crashdumpCmdFlags struct {
	clusterState clusterNodes
}

// CrashdumpCmd represents the crashdump command.
var CrashdumpCmd = &cobra.Command{
	Use:    "crashdump",
	Short:  "Dump debug information about the cluster",
	Long:   ``,
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			return errors.New("`talosctl crashdump` is deprecated, please use `talosctl support` instead")
		})
	},
}

func init() {
	addCommand(CrashdumpCmd)
	CrashdumpCmd.Flags().StringVar(&crashdumpCmdFlags.clusterState.InitNode, "init-node", "", "specify IPs of init node")
	CrashdumpCmd.Flags().StringSliceVar(&crashdumpCmdFlags.clusterState.ControlPlaneNodes, "control-plane-nodes", nil, "specify IPs of control plane nodes")
	CrashdumpCmd.Flags().StringSliceVar(&crashdumpCmdFlags.clusterState.WorkerNodes, "worker-nodes", nil, "specify IPs of worker nodes")
}
