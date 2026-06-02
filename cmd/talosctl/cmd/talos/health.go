// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/cluster/check"
	"github.com/siderolabs/talos/pkg/cluster/hydrophone"
	clusterapi "github.com/siderolabs/talos/pkg/machinery/api/cluster"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	clusterres "github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

type clusterNodes struct {
	InitNode          string
	ControlPlaneNodes []string
	WorkerNodes       []string

	nodes       []cluster.NodeInfo
	nodesByType map[machine.Type][]cluster.NodeInfo
}

func (cl *clusterNodes) InitNodeInfos() error {
	var initNodes []string

	if cl.InitNode != "" {
		initNodes = []string{cl.InitNode}
	}

	initNodeInfos, err := cluster.IPsToNodeInfos(initNodes)
	if err != nil {
		return err
	}

	controlPlaneNodeInfos, err := cluster.IPsToNodeInfos(cl.ControlPlaneNodes)
	if err != nil {
		return err
	}

	workerNodeInfos, err := cluster.IPsToNodeInfos(cl.WorkerNodes)
	if err != nil {
		return err
	}

	nodesByType := make(map[machine.Type][]cluster.NodeInfo)
	nodesByType[machine.TypeInit] = initNodeInfos
	nodesByType[machine.TypeControlPlane] = controlPlaneNodeInfos
	nodesByType[machine.TypeWorker] = workerNodeInfos
	cl.nodesByType = nodesByType

	cl.nodes = slices.Concat(initNodeInfos, controlPlaneNodeInfos, workerNodeInfos)

	return nil
}

func (cl *clusterNodes) Nodes() []cluster.NodeInfo {
	return cl.nodes
}

func (cl *clusterNodes) NodesByType(t machine.Type) []cluster.NodeInfo {
	return cl.nodesByType[t]
}

var healthCmdFlags struct {
	clusterState       clusterNodes
	clusterWaitTimeout time.Duration
	forceEndpoint      string
	runOnServer        bool
	runE2E             bool
}

// healthCmd represents the health command.
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check cluster health",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := healthCmdFlags.clusterState.InitNodeInfos()
		if err != nil {
			return err
		}

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &healthCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		if err := runHealth(ctx, clientFactory); err != nil {
			return err
		}

		if healthCmdFlags.runE2E {
			return runE2E(ctx, clientFactory)
		}

		return nil
	},
}

func runHealth(ctx context.Context, clientFactory *global.ClientFactory) error {
	if healthCmdFlags.runOnServer {
		return healthOnServer(ctx, clientFactory)
	}

	return healthOnClient(ctx, clientFactory)
}

func healthOnClient(ctx context.Context, clientFactory *global.ClientFactory) error {
	ctx, c, _, err := clientFactory.BuildClientEnforceSingleNode(ctx, "health")
	if err != nil {
		return err
	}

	clientProvider := &cluster.ConfigClientProvider{
		DefaultClient: c,
	}
	defer clientProvider.Close() //nolint:errcheck

	clusterInfo, err := buildClusterInfo(ctx, c, healthCmdFlags.clusterState)
	if err != nil {
		return err
	}

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
		Info: clusterInfo,
	}

	// Run cluster readiness checks
	checkCtx, checkCtxCancel := context.WithTimeout(ctx, healthCmdFlags.clusterWaitTimeout)
	defer checkCtxCancel()

	return check.Wait(checkCtx, &state, append(check.DefaultClusterChecks(), check.ExtraClusterChecks()...), check.StderrReporter())
}

func healthOnServer(ctx context.Context, clientFactory *global.ClientFactory) error {
	ctx, c, _, err := clientFactory.BuildClientEnforceSingleNode(ctx, "health")
	if err != nil {
		return err
	}

	controlPlaneNodes := healthCmdFlags.clusterState.ControlPlaneNodes
	if healthCmdFlags.clusterState.InitNode != "" {
		controlPlaneNodes = append(controlPlaneNodes, healthCmdFlags.clusterState.InitNode)
	}

	healthCheckClient, err := c.ClusterHealthCheck(ctx, healthCmdFlags.clusterWaitTimeout, &clusterapi.ClusterInfo{
		ControlPlaneNodes: controlPlaneNodes,
		WorkerNodes:       healthCmdFlags.clusterState.WorkerNodes,
		ForceEndpoint:     healthCmdFlags.forceEndpoint,
	})
	if err != nil {
		return err
	}

	if err := healthCheckClient.CloseSend(); err != nil {
		return err
	}

	for {
		msg, err := healthCheckClient.Recv()
		if err != nil {
			if err == io.EOF || client.StatusCode(err) == codes.Canceled {
				return nil
			}

			return err
		}

		fmt.Fprintln(os.Stderr, msg.GetMessage())
	}
}

func runE2E(ctx context.Context, clientFactory *global.ClientFactory) error {
	ctx, c, _, err := clientFactory.BuildClientEnforceSingleNode(ctx, "health")
	if err != nil {
		return err
	}

	clientProvider := &cluster.ConfigClientProvider{
		DefaultClient: c,
	}
	defer clientProvider.Close() //nolint:errcheck

	state := &cluster.KubernetesClient{
		ClientProvider: clientProvider,
		ForceEndpoint:  healthCmdFlags.forceEndpoint,
	}

	// Run cluster readiness checks
	checkCtx, checkCtxCancel := context.WithTimeout(ctx, healthCmdFlags.clusterWaitTimeout)
	defer checkCtxCancel()

	options := hydrophone.DefaultOptions()
	options.UseSpinner = true

	return hydrophone.Run(checkCtx, state, options)
}

func init() {
	addCommand(healthCmd)
	healthCmd.Flags().StringVar(&healthCmdFlags.clusterState.InitNode, "init-node", "", "specify IPs of init node")
	healthCmd.Flags().StringSliceVar(&healthCmdFlags.clusterState.ControlPlaneNodes, "control-plane-nodes", nil, "specify IPs of control plane nodes")
	healthCmd.Flags().StringSliceVar(&healthCmdFlags.clusterState.WorkerNodes, "worker-nodes", nil, "specify IPs of worker nodes")
	healthCmd.Flags().DurationVar(&healthCmdFlags.clusterWaitTimeout, "wait-timeout", 20*time.Minute, "timeout to wait for the cluster to be ready")
	healthCmd.Flags().StringVar(&healthCmdFlags.forceEndpoint, "k8s-endpoint", "", "use endpoint instead of kubeconfig default")
	healthCmd.Flags().BoolVar(&healthCmdFlags.runOnServer, "server", true, "run server-side check")
	healthCmd.Flags().BoolVar(&healthCmdFlags.runE2E, "run-e2e", false, "run Kubernetes e2e test")
}

func buildClusterInfo(ctx context.Context, c *client.Client, clusterState clusterNodes) (cluster.Info, error) {
	// if nodes are set explicitly via command line args, use them
	if len(clusterState.ControlPlaneNodes) > 0 || len(clusterState.WorkerNodes) > 0 {
		return &clusterState, nil
	}

	// read members from the Talos API
	var members []*clusterres.Member

	items, err := safe.StateListAll[*clusterres.Member](ctx, c.COSI)
	if err != nil {
		return nil, err
	}

	items.ForEach(func(item *clusterres.Member) { members = append(members, item) })

	return check.NewDiscoveredClusterInfo(members)
}
