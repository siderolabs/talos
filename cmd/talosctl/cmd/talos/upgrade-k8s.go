// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/cluster"
	k8s "github.com/talos-systems/talos/pkg/cluster/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// upgradeK8sCmd represents the upgrade-k8s command.
var upgradeK8sCmd = &cobra.Command{
	Use:   "upgrade-k8s",
	Short: "Upgrade Kubernetes control plane in the Talos cluster.",
	Long:  `Command runs upgrade of Kubernetes control plane components between specified versions. Pod-checkpointer is handled in a special way to speed up kube-apisever upgrades.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(upgradeKubernetes)
	},
}

var upgradeOptions k8s.UpgradeOptions

func init() {
	upgradeK8sCmd.Flags().StringVar(&upgradeOptions.FromVersion, "from", "", "the Kubernetes control plane version to upgrade from")
	upgradeK8sCmd.Flags().StringVar(&upgradeOptions.ToVersion, "to", constants.DefaultKubernetesVersion, "the Kubernetes control plane version to upgrade to")
	upgradeK8sCmd.Flags().StringVar(&upgradeOptions.ControlPlaneEndpoint, "endpoint", "", "the cluster control plane endpoint")
	cli.Should(upgradeK8sCmd.MarkFlagRequired("from"))
	cli.Should(upgradeK8sCmd.MarkFlagRequired("to"))
	addCommand(upgradeK8sCmd)
}

func upgradeKubernetes(ctx context.Context, c *client.Client) error {
	clientProvider := &cluster.ConfigClientProvider{
		DefaultClient: c,
	}
	defer clientProvider.Close() //nolint:errcheck

	state := struct {
		cluster.ClientProvider
		cluster.K8sProvider
	}{
		ClientProvider: clientProvider,
		K8sProvider: &cluster.KubernetesClient{
			ClientProvider: clientProvider,
			ForceEndpoint:  upgradeOptions.ControlPlaneEndpoint,
		},
	}

	selfHosted, err := k8s.IsSelfHostedControlPlane(ctx, &state, Nodes[0])
	if err != nil {
		return fmt.Errorf("error checking self-hosted status: %w", err)
	}

	if selfHosted {
		return k8s.UpgradeSelfHosted(ctx, &state, upgradeOptions)
	}

	return k8s.UpgradeTalosManaged(ctx, &state, upgradeOptions)
}
