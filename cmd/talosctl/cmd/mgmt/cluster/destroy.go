// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

var destroyCmdFlags struct {
	forceDelete                bool
	saveSupportArchivePath     string
	saveClusterLogsArchivePath string
}

// destroyCmd represents the cluster destroy command.
var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroys a local docker-based or firecracker-based kubernetes cluster",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.WithContext(context.Background(), destroy)
	},
}

func destroy(ctx context.Context) error {
	provisioner, err := providers.Factory(ctx, Flags.ProvisionerName)
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint:errcheck

	cluster, err := provisioner.Reflect(ctx, Flags.ClusterName, Flags.StateDir)
	if err != nil {
		return err
	}

	return provisioner.Destroy(
		ctx,
		cluster,
		provision.WithDeleteOnErr(destroyCmdFlags.forceDelete),
		provision.WithSaveSupportArchivePath(destroyCmdFlags.saveSupportArchivePath),
		provision.WithSaveClusterLogsArchivePath(destroyCmdFlags.saveClusterLogsArchivePath),
	)
}

func init() {
	destroyCmd.PersistentFlags().BoolVarP(&destroyCmdFlags.forceDelete, "force", "f", false, "force deletion of cluster directory if there were errors")
	destroyCmd.PersistentFlags().StringVarP(&destroyCmdFlags.saveSupportArchivePath, "save-support-archive-path", "", "", "save support archive to the specified file on destroy")
	destroyCmd.PersistentFlags().StringVarP(&destroyCmdFlags.saveClusterLogsArchivePath, "save-cluster-logs-archive-path", "", "", "save cluster logs archive to the specified file on destroy")

	Cmd.AddCommand(destroyCmd)
}
