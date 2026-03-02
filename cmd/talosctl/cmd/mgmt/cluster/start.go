// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers"
)

// startCmd represents the cluster start command.
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts a stopped local Talos Kubernetes cluster",
	Long: `Starts a local Talos Kubernetes cluster that was previously created but is now stopped.
This is useful when the development container is restarted and the VM processes are no longer running.
The cluster state (disks, configs) must still exist from a previous 'cluster create' command.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.WithContext(context.Background(), start)
	},
}

func start(ctx context.Context) error {
	state, err := provision.ReadState(ctx, PersistentFlags.ClusterName, PersistentFlags.StateDir)
	if err != nil {
		return fmt.Errorf("failed to read cluster state: %w", err)
	}

	provisioner, err := providers.Factory(ctx, state.ProvisionerName)
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint:errcheck

	cluster, err := provisioner.Reflect(ctx, PersistentFlags.ClusterName, PersistentFlags.StateDir)
	if err != nil {
		return err
	}

	return provisioner.Start(
		ctx,
		cluster,
		provision.WithLogWriter(os.Stdout),
	)
}

func init() {
	AddProvisionerFlag(startCmd)
	cli.Should(startCmd.Flags().MarkDeprecated(ProvisionerFlagName, "the provisioner is inferred automatically"))

	Cmd.AddCommand(startCmd)
}
