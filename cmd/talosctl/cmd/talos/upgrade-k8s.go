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
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// UpgradeK8sCmd represents the upgrade-k8s command.
var UpgradeK8sCmd = &cobra.Command{
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

	withExamples bool
	withDocs     bool
}

func init() {
	UpgradeK8sCmd.Flags().StringVar(&upgradeK8sCmdFlags.FromVersion, "from", "", "the Kubernetes control plane version to upgrade from")
	UpgradeK8sCmd.Flags().StringVar(&upgradeK8sCmdFlags.ToVersion, "to", constants.DefaultKubernetesVersion, "the Kubernetes control plane version to upgrade to")
	UpgradeK8sCmd.Flags().StringVar(&upgradeOptions.ControlPlaneEndpoint, "endpoint", "", "the cluster control plane endpoint")
	UpgradeK8sCmd.Flags().BoolVarP(&upgradeK8sCmdFlags.withExamples, "with-examples", "", true, "patch all machine configs with the commented examples")
	UpgradeK8sCmd.Flags().BoolVarP(&upgradeK8sCmdFlags.withDocs, "with-docs", "", true, "patch all machine configs adding the documentation for each field")
	UpgradeK8sCmd.Flags().BoolVar(&upgradeOptions.DryRun, "dry-run", false, "skip the actual upgrade and show the upgrade plan instead")
	UpgradeK8sCmd.Flags().BoolVar(&upgradeOptions.PrePullImages, "pre-pull-images", true, "pre-pull images before upgrade")
	UpgradeK8sCmd.Flags().BoolVar(&upgradeOptions.UpgradeKubelet, "upgrade-kubelet", true, "upgrade kubelet service")

	UpgradeK8sCmd.Flags().StringVar(&upgradeOptions.KubeletImage, "kubelet-image", constants.KubeletImage, "kubelet image to use")
	UpgradeK8sCmd.Flags().StringVar(&upgradeOptions.APIServerImage, "apiserver-image", constants.KubernetesAPIServerImage, "kube-apiserver image to use")
	UpgradeK8sCmd.Flags().StringVar(&upgradeOptions.ControllerManagerImage, "controller-manager-image", constants.KubernetesControllerManagerImage, "kube-controller-manager image to use")
	UpgradeK8sCmd.Flags().StringVar(&upgradeOptions.SchedulerImage, "scheduler-image", constants.KubernetesSchedulerImage, "kube-scheduler image to use")
	UpgradeK8sCmd.Flags().StringVar(&upgradeOptions.ProxyImage, "proxy-image", constants.KubeProxyImage, "kube-proxy image to use")

	addCommand(UpgradeK8sCmd)
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

	upgradeOptions.EncoderOpt = encoder.WithComments(commentsFlags)

	return k8s.Upgrade(ctx, &state, upgradeOptions)
}
