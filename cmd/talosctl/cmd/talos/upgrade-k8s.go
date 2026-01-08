// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/siderolabs/go-kubernetes/kubernetes/manifests"
	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
	"github.com/spf13/cobra"
	"sigs.k8s.io/cli-utils/pkg/inventory"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/cluster"
	k8s "github.com/siderolabs/talos/pkg/cluster/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
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

	withExamples    bool
	withDocs        bool
	inventoryPolicy string
}

func init() {
	ssaDefaults := manifests.DefaultSSApplyBehaviorOptions()

	upgradeK8sCmd.Flags().StringVar(&upgradeK8sCmdFlags.FromVersion, "from", "", "the Kubernetes control plane version to upgrade from")
	upgradeK8sCmd.Flags().StringVar(&upgradeK8sCmdFlags.ToVersion, "to", constants.DefaultKubernetesVersion, "the Kubernetes control plane version to upgrade to")
	upgradeK8sCmd.Flags().StringVar(&upgradeOptions.ControlPlaneEndpoint, "endpoint", "", "the cluster control plane endpoint")
	upgradeK8sCmd.Flags().BoolVarP(&upgradeK8sCmdFlags.withExamples, "with-examples", "", true, "patch all machine configs with the commented examples")
	upgradeK8sCmd.Flags().BoolVarP(&upgradeK8sCmdFlags.withDocs, "with-docs", "", true, "patch all machine configs adding the documentation for each field")
	upgradeK8sCmd.Flags().BoolVar(&upgradeOptions.PrePullImages, "pre-pull-images", true, "pre-pull images before upgrade")
	upgradeK8sCmd.Flags().BoolVar(&upgradeOptions.UpgradeKubelet, "upgrade-kubelet", true, "upgrade kubelet service")
	upgradeK8sCmd.Flags().BoolVar(&upgradeOptions.DryRun, "dry-run", false, "skip the actual upgrade and show the upgrade plan instead")

	upgradeK8sCmd.Flags().StringVar(&upgradeOptions.KubeletImage, "kubelet-image", constants.KubeletImage, "kubelet image to use")
	upgradeK8sCmd.Flags().StringVar(&upgradeOptions.APIServerImage, "apiserver-image", constants.KubernetesAPIServerImage, "kube-apiserver image to use")
	upgradeK8sCmd.Flags().StringVar(&upgradeOptions.ControllerManagerImage, "controller-manager-image", constants.KubernetesControllerManagerImage, "kube-controller-manager image to use")
	upgradeK8sCmd.Flags().StringVar(&upgradeOptions.SchedulerImage, "scheduler-image", constants.KubernetesSchedulerImage, "kube-scheduler image to use")
	upgradeK8sCmd.Flags().StringVar(&upgradeOptions.ProxyImage, "proxy-image", constants.KubeProxyImage, "kube-proxy image to use")

	// manifest sync related options
	upgradeK8sCmd.Flags().BoolVar(&upgradeOptions.ForceConflicts, "manifests-force-conflicts", ssaDefaults.ForceConflicts, "overwrite the fields when applying even if the field manager differs")
	upgradeK8sCmd.Flags().BoolVar(&upgradeOptions.NoPrune, "manifests-no-prune", ssaDefaults.NoPrune, "whether pruning of previously applied objects should happen after apply")
	upgradeK8sCmd.Flags().StringVar(&upgradeK8sCmdFlags.inventoryPolicy, "manifests-inventory-policy", ssaDefaults.InventoryPolicy.String(),
		"kubernetes SSA inventory policy (one of 'MustMatch', 'AdoptIfNoInventory' or 'AdoptAll')")
	upgradeK8sCmd.Flags().DurationVar(&upgradeOptions.PruneTimeout, "manifests-prune-timeout", ssaDefaults.PruneTimeout,
		"how long to wait for resources to be fully deleted (set to zero to disable waiting)")
	upgradeK8sCmd.Flags().DurationVar(&upgradeOptions.ReconcileTimeout, "manifests-reconcile-timeout", ssaDefaults.ReconcileTimeout,
		"how long to wait for resources to be fully reconciled (set to zero to disable waiting)")

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

	commentsFlags := encoder.CommentsDisabled
	if upgradeK8sCmdFlags.withDocs {
		commentsFlags |= encoder.CommentsDocs
	}

	if upgradeK8sCmdFlags.withExamples {
		commentsFlags |= encoder.CommentsExamples
	}

	policy, err := parseInventoryPolicy(upgradeK8sCmdFlags.inventoryPolicy)
	if err != nil {
		return err
	}

	upgradeOptions.InventoryPolicy = policy
	upgradeOptions.EncoderOpt = encoder.WithComments(commentsFlags)

	return k8s.Upgrade(ctx, &state, upgradeOptions)
}

func parseInventoryPolicy(policy string) (inventory.Policy, error) {
	switch policy {
	case "MustMatch":
		return inventory.PolicyMustMatch, nil
	case "AdoptIfNoInventory":
		return inventory.PolicyAdoptIfNoInventory, nil
	case "AdoptAll":
		return inventory.PolicyAdoptAll, nil
	default:
		return 0, fmt.Errorf("invalid inventory policy %q: must be one of 'MustMatch', 'AdoptIfNoInventory', or 'AdoptAll'", policy)
	}
}
