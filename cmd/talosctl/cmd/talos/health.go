// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	clusterapi "github.com/talos-systems/talos/api/cluster"
	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/internal/pkg/cluster"
	"github.com/talos-systems/talos/internal/pkg/cluster/check"
	"github.com/talos-systems/talos/pkg/client"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/machine"
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
	case machine.TypeJoin:
		return cluster.WorkerNodes
	default:
		panic("unsupported machine type")
	}
}

var healthCmdFlags struct {
	clusterState       clusterNodes
	clusterWaitTimeout time.Duration
	forceEndpoint      string
	runOnServer        bool
}

// healthCmd represents the health command.
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check cluster health",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if healthCmdFlags.runOnServer {
			return WithClient(healthOnServer)
		}

		return WithClientNoNodes(healthOnClient)
	},
}

func healthOnClient(ctx context.Context, c *client.Client) error {
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
			ForceEndpoint:  healthCmdFlags.forceEndpoint,
		},
		Info: &healthCmdFlags.clusterState,
	}

	// Run cluster readiness checks
	checkCtx, checkCtxCancel := context.WithTimeout(ctx, healthCmdFlags.clusterWaitTimeout)
	defer checkCtxCancel()

	return check.Wait(checkCtx, &state, append(check.DefaultClusterChecks(), check.ExtraClusterChecks()...), check.StderrReporter())
}

func healthOnServer(ctx context.Context, c *client.Client) error {
	if err := helpers.FailIfMultiNodes(ctx, "health"); err != nil {
		return err
	}

	controlPlaneNodes := healthCmdFlags.clusterState.ControlPlaneNodes
	if healthCmdFlags.clusterState.InitNode != "" {
		controlPlaneNodes = append(controlPlaneNodes, healthCmdFlags.clusterState.InitNode)
	}

	client, err := c.ClusterHealthCheck(ctx, healthCmdFlags.clusterWaitTimeout, &clusterapi.ClusterInfo{
		ControlPlaneNodes: controlPlaneNodes,
		WorkerNodes:       healthCmdFlags.clusterState.WorkerNodes,
		ForceEndpoint:     healthCmdFlags.forceEndpoint,
	})
	if err != nil {
		return err
	}

	if err := client.CloseSend(); err != nil {
		return err
	}

	for {
		msg, err := client.Recv()
		if err != nil {
			if err == io.EOF || status.Code(err) == codes.Canceled {
				return nil
			}

			return err
		}

		if msg.GetMetadata().GetError() != "" {
			return fmt.Errorf("healthcheck error: %s", msg.GetMetadata().GetError())
		}

		fmt.Fprintln(os.Stderr, msg.GetMessage())
	}
}

func init() {
	addCommand(healthCmd)
	healthCmd.Flags().StringVar(&healthCmdFlags.clusterState.InitNode, "init-node", "", "specify IPs of init node")
	healthCmd.Flags().StringSliceVar(&healthCmdFlags.clusterState.ControlPlaneNodes, "control-plane-nodes", nil, "specify IPs of control plane nodes")
	healthCmd.Flags().StringSliceVar(&healthCmdFlags.clusterState.WorkerNodes, "worker-nodes", nil, "specify IPs of worker nodes")
	healthCmd.Flags().DurationVar(&healthCmdFlags.clusterWaitTimeout, "wait-timeout", 20*time.Minute, "timeout to wait for the cluster to be ready")
	healthCmd.Flags().StringVar(&healthCmdFlags.forceEndpoint, "k8s-endpoint", "", "use endpoint instead of kubeconfig default")
	healthCmd.Flags().BoolVar(&healthCmdFlags.runOnServer, "server", true, "run server-side check")
}
