// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/provision/providers"
)

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
	provisioner, err := providers.Factory(ctx, provisionerName)
	if err != nil {
		return err
	}

	defer provisioner.Close() //nolint:errcheck

	cluster, err := provisioner.Reflect(ctx, clusterName, stateDir)
	if err != nil {
		return err
	}

	return provisioner.Destroy(ctx, cluster)
}

func init() {
	Cmd.AddCommand(destroyCmd)
}
