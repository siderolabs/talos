// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/cluster"
	k8s "github.com/siderolabs/talos/pkg/cluster/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// upgradeK8sCmd represents the upgrade-k8s command.
var upgradeK8sCmd = &cobra.Command{
	Use:   "upgrade-k8s",
	Short: "Upgrade Kubernetes control plane in the Talos cluster.",
	Long:  `Command runs upgrade of Kubernetes control plane components between specified versions.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(upgradeKubernetes)
	},
}

var upgradeOptions k8s.UpgradeOptions

var upgradeK8sCmdFlags struct {
	FromVersion string
	ToVersion   string
}

func init() {
	upgradeK8sCmd.Flags().StringVar(&upgradeK8sCmdFlags.FromVersion, "from", "", "the Kubernetes control plane version to upgrade from")
	upgradeK8sCmd.Flags().StringVar(&upgradeK8sCmdFlags.ToVersion, "to", constants.DefaultKubernetesVersion, "the Kubernetes control plane version to upgrade to")
	upgradeK8sCmd.Flags().StringVar(&upgradeOptions.ControlPlaneEndpoint, "endpoint", "", "the cluster control plane endpoint")
	upgradeK8sCmd.Flags().BoolVar(&upgradeOptions.DryRun, "dry-run", false, "skip the actual upgrade and show the upgrade plan instead")
	upgradeK8sCmd.Flags().BoolVar(&upgradeOptions.UpgradeKubelet, "upgrade-kubelet", true, "upgrade kubelet service")
	addCommand(upgradeK8sCmd)
}

func upgradeKubernetes(ctx context.Context, c *client.Client) error {
	if err := helpers.FailIfMultiNodes(ctx, "upgrade-k8s"); err != nil {
		return err
	}

	if err := helpers.ClientVersionCheck(ctx, c); err != nil {
		return err
	}

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

	var err error

	if upgradeK8sCmdFlags.FromVersion == "" {
		upgradeK8sCmdFlags.FromVersion, err = k8s.DetectLowestVersion(ctx, &state, upgradeOptions)
		if err != nil {
			return fmt.Errorf("error detecting the lowest Kubernetes version %w", err)
		}

		upgradeOptions.Log("automatically detected the lowest Kubernetes version %s", upgradeK8sCmdFlags.FromVersion)
	}

	upgradeOptions.Path, err = upgrade.NewPath(upgradeK8sCmdFlags.FromVersion, upgradeK8sCmdFlags.ToVersion)
	if err != nil {
		return fmt.Errorf("error creating upgrade path %w", err)
	}

	return k8s.Upgrade(ctx, &state, upgradeOptions)
}
