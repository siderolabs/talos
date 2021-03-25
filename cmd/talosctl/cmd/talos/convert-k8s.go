// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/cluster"
	k8s "github.com/talos-systems/talos/pkg/cluster/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// convertK8sCmd represents the convert-k8s command.
var convertK8sCmd = &cobra.Command{
	Use:   "convert-k8s",
	Short: "Convert Kubernetes control plane from self-hosted (bootkube) to Talos-managed (static pods).",
	Long: `Command converts control plane bootstrapped on Talos <= 0.8 to Talos-managed control plane (Talos >= 0.9).
As part of the conversion process tool reads existing configuration of the control plane, updates
Talos node configuration to reflect changes made since the boostrap time. Once config is updated,
tool releases static pods and deletes self-hosted DaemonSets.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(Nodes) > 0 {
			convertOptions.Node = Nodes[0]
		}

		return WithClient(convertKubernetes)
	},
}

var convertOptions k8s.ConvertOptions

func init() {
	convertK8sCmd.Flags().StringVar(&convertOptions.ControlPlaneEndpoint, "endpoint", "", "the cluster control plane endpoint")
	convertK8sCmd.Flags().BoolVar(&convertOptions.ForceYes, "force", false, "skip prompts, assume yes")
	convertK8sCmd.Flags().BoolVar(&convertOptions.OnlyRemoveInitializedKey, "remove-initialized-key", false, "only remove bootkube initialized key (used in manual conversion)")

	// hiding this flag as it should only be used in manual process (and it's documented there), but should never be used in automatic conversion
	convertK8sCmd.Flags().MarkHidden("remove-initialized-key") //nolint:errcheck

	addCommand(convertK8sCmd)
}

func convertKubernetes(ctx context.Context, c *client.Client) error {
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
			ForceEndpoint:  convertOptions.ControlPlaneEndpoint,
		},
	}

	return k8s.ConvertToStaticPods(ctx, &state, convertOptions)
}
