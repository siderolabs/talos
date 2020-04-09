// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/internal/pkg/cluster"
	"github.com/talos-systems/talos/internal/pkg/cluster/check"
	"github.com/talos-systems/talos/pkg/client"
	"github.com/talos-systems/talos/pkg/config/machine"
)

type clusterNodes struct {
	InitNode          string
	ControlPlaneNodes []string
	WorkerNodes       []string
}

func (cluster *clusterNodes) Nodes() []string {
	return append([]string{cluster.InitNode}, append(cluster.ControlPlaneNodes, cluster.WorkerNodes...)...)
}

func (cluster *clusterNodes) NodesByType(t machine.Type) []string {
	switch t {
	case machine.TypeInit:
		return []string{cluster.InitNode}
	case machine.TypeControlPlane:
		return cluster.ControlPlaneNodes
	case machine.TypeWorker:
		return cluster.WorkerNodes
	default:
		panic("unsupported machine type")
	}
}

var (
	clusterState       clusterNodes
	clusterWaitTimeout time.Duration
	forceEndpoint      string
)

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check cluster health",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			clientProvider := &cluster.ConfigClientProvider{
				DefaultClient: c,
			}
			defer clientProvider.Close() //nolint: errcheck

			state := struct {
				cluster.ClientProvider
				cluster.K8sProvider
				cluster.Info
			}{
				ClientProvider: clientProvider,
				K8sProvider: &cluster.KubernetesClient{
					ClientProvider: clientProvider,
					ForceEndpoint:  forceEndpoint,
				},
				Info: &clusterState,
			}

			// Run cluster readiness checks
			checkCtx, checkCtxCancel := context.WithTimeout(ctx, clusterWaitTimeout)
			defer checkCtxCancel()

			return check.Wait(checkCtx, &state, check.DefaultClusterChecks(), check.StderrReporter())
		})
	},
}

func init() {
	addCommand(healthCmd)
	healthCmd.Flags().StringVar(&clusterState.InitNode, "init-node", "", "specify IPs of init node")
	healthCmd.Flags().StringSliceVar(&clusterState.ControlPlaneNodes, "control-plane-nodes", nil, "specify IPs of control plane nodes")
	healthCmd.Flags().StringSliceVar(&clusterState.WorkerNodes, "worker-nodes", nil, "specify IPs of worker nodes")
	healthCmd.Flags().DurationVar(&clusterWaitTimeout, "wait-timeout", 20*time.Minute, "timeout to wait for the cluster to be ready")
	healthCmd.Flags().StringVar(&forceEndpoint, "k8s-endpoint", "", "use endpoint instead of kubeconfig default")
}
