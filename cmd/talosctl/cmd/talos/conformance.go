// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/cluster/sonobuoy"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// conformanceCmd represents the conformance command.
var conformanceCmd = &cobra.Command{
	Use:   "conformance",
	Short: "Run conformance tests",
	Long:  ``,
}

var conformanceKubernetesCmdFlags struct {
	mode string
}

var conformanceKubernetesCmd = &cobra.Command{
	Use:     "kubernetes",
	Aliases: []string{"k8s"},
	Short:   "Run Kubernetes conformance tests",
	Long:    ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			clientProvider := &cluster.ConfigClientProvider{
				DefaultClient: c,
			}
			defer clientProvider.Close() //nolint:errcheck

			state := struct {
				cluster.K8sProvider
			}{
				K8sProvider: &cluster.KubernetesClient{
					ClientProvider: clientProvider,
					ForceEndpoint:  healthCmdFlags.forceEndpoint,
				},
			}

			switch conformanceKubernetesCmdFlags.mode {
			case "fast":
				return sonobuoy.FastConformance(ctx, &state)
			case "certified":
				return sonobuoy.CertifiedConformance(ctx, &state)
			default:
				return fmt.Errorf("unsupported conformance mode %v", conformanceKubernetesCmdFlags.mode)
			}
		})
	},
}

func init() {
	conformanceKubernetesCmd.Flags().StringVar(&conformanceKubernetesCmdFlags.mode, "mode", "fast", "conformance test mode: [fast, certified]")
	conformanceCmd.AddCommand(conformanceKubernetesCmd)
	addCommand(conformanceCmd)
}
