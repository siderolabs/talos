// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.etcd.io/etcd/etcdctl/v3/snapshot"

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var bootstrapCmdFlags struct {
	recoverFrom          string
	recoverSkipHashCheck bool
}

// bootstrapCmd represents the bootstrap command.
var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstrap the etcd cluster on the specified node.",
	Long: `When Talos cluster is created etcd service on control plane nodes enter the join loop waiting
to join etcd peers from other control plane nodes. One node should be picked as the boostrap node.
When boostrap command is issued, the node aborts join process and bootstraps etcd cluster as a single node cluster.
Other control plane nodes will join etcd cluster once Kubernetes is boostrapped on the bootstrap node.

This command should not be used when "init" type node are used.

Talos etcd cluster can be recovered from a known snapshot with '--recover-from=' flag.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			if bootstrapCmdFlags.recoverFrom != "" {
				manager := snapshot.NewV3(nil)

				status, err := manager.Status(bootstrapCmdFlags.recoverFrom)
				if err != nil {
					return err
				}

				fmt.Printf("recovering from snapshot %q: hash %08x, revision %d, total keys %d, total size %d\n",
					bootstrapCmdFlags.recoverFrom, status.Hash, status.Revision, status.TotalKey, status.TotalSize)

				snapshot, err := os.Open(bootstrapCmdFlags.recoverFrom)
				if err != nil {
					return fmt.Errorf("error opening snapshot file: %w", err)
				}

				defer snapshot.Close() //nolint: errcheck

				_, err = c.EtcdRecover(ctx, snapshot)
				if err != nil {
					return fmt.Errorf("error uploading snapshot: %w", err)
				}
			}

			if err := c.Bootstrap(ctx, &machineapi.BootstrapRequest{
				RecoverEtcd:          bootstrapCmdFlags.recoverFrom != "",
				RecoverSkipHashCheck: bootstrapCmdFlags.recoverSkipHashCheck,
			}); err != nil {
				return fmt.Errorf("error executing bootstrap: %s", err)
			}

			return nil
		})
	},
}

func init() {
	bootstrapCmd.Flags().StringVar(&bootstrapCmdFlags.recoverFrom, "recover-from", "", "recover etcd cluster from the snapshot")
	bootstrapCmd.Flags().BoolVar(&bootstrapCmdFlags.recoverSkipHashCheck, "recover-skip-hash-check", false, "skip integrity check when recovering etcd (use when recovering from data directory copy)")
	addCommand(bootstrapCmd)
}
