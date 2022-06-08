// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/metadata"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/cluster/check"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/grpc/middleware/authz"
	clusterapi "github.com/talos-systems/talos/pkg/machinery/api/cluster"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// HealthCheck implements the cluster.ClusterServer interface.
func (s *Server) HealthCheck(in *clusterapi.HealthCheckRequest, srv clusterapi.ClusterService_HealthCheckServer) error {
	clientProvider := &cluster.LocalClientProvider{}
	defer clientProvider.Close() //nolint:errcheck

	k8sProvider := &cluster.KubernetesClient{
		ClientProvider: clientProvider,
		ForceEndpoint:  in.GetClusterInfo().GetForceEndpoint(),
	}
	defer k8sProvider.K8sClose() //nolint:errcheck

	controlPlaneNodes := in.GetClusterInfo().GetControlPlaneNodes()
	workerNodes := in.GetClusterInfo().GetWorkerNodes()

	controlPlaneNodeInfos, err := cluster.IPsToNodeInfos(controlPlaneNodes)
	if err != nil {
		return err
	}

	workerNodeInfos, err := cluster.IPsToNodeInfos(workerNodes)
	if err != nil {
		return err
	}

	clusterState := clusterState{
		controlPlaneNodes: controlPlaneNodes,
		workerNodes:       workerNodes,
		nodeInfos:         append(controlPlaneNodeInfos, workerNodeInfos...),
		nodeInfosByType: map[machine.Type][]cluster.NodeInfo{
			machine.TypeControlPlane: controlPlaneNodeInfos,
			machine.TypeWorker:       workerNodeInfos,
		},
	}

	state := struct {
		cluster.ClientProvider
		cluster.K8sProvider
		cluster.Info
	}{
		ClientProvider: clientProvider,
		K8sProvider:    k8sProvider,
		Info:           &clusterState,
	}

	// Run cluster readiness checks

	checkCtx, checkCtxCancel := context.WithTimeout(srv.Context(), in.WaitTimeout.AsDuration())
	defer checkCtxCancel()

	md := metadata.New(nil)
	authz.SetMetadata(md, authz.GetRoles(srv.Context()))
	checkCtx = metadata.NewOutgoingContext(checkCtx, md)

	if err := clusterState.resolve(checkCtx, k8sProvider); err != nil {
		return fmt.Errorf("error discovering nodes: %w", err)
	}

	if err := srv.Send(&clusterapi.HealthCheckProgress{
		Message: fmt.Sprintf("discovered nodes: %s", &clusterState),
	}); err != nil {
		return err
	}

	return check.Wait(checkCtx, &state, append(check.DefaultClusterChecks(), check.ExtraClusterChecks()...), &healthReporter{srv: srv})
}

type healthReporter struct {
	srv      clusterapi.ClusterService_HealthCheckServer
	lastLine string
}

func (hr *healthReporter) Update(condition conditions.Condition) {
	line := fmt.Sprintf("waiting for %s", condition)

	if line != hr.lastLine {
		hr.srv.Send(&clusterapi.HealthCheckProgress{ //nolint:errcheck
			Message: strings.TrimSpace(line),
		})

		hr.lastLine = line
	}
}

type clusterState struct {
	controlPlaneNodes []string
	workerNodes       []string

	nodeInfos       []cluster.NodeInfo
	nodeInfosByType map[machine.Type][]cluster.NodeInfo
}

func (cl *clusterState) Nodes() []cluster.NodeInfo {
	return cl.nodeInfos
}

func (cl *clusterState) NodesByType(t machine.Type) []cluster.NodeInfo {
	return cl.nodeInfosByType[t]
}

func (cl *clusterState) resolve(ctx context.Context, k8sProvider *cluster.KubernetesClient) error {
	if len(cl.controlPlaneNodes) == 0 && len(cl.workerNodes) == 0 {
		var err error

		if _, err = k8sProvider.K8sClient(ctx); err != nil {
			return err
		}

		if cl.controlPlaneNodes, err = k8sProvider.KubeHelper.NodeIPs(ctx, machine.TypeControlPlane); err != nil {
			return err
		}

		if cl.workerNodes, err = k8sProvider.KubeHelper.NodeIPs(ctx, machine.TypeWorker); err != nil {
			return err
		}

		controlPlaneNodeInfos, err := cluster.IPsToNodeInfos(cl.controlPlaneNodes)
		if err != nil {
			return err
		}

		workerNodeInfos, err := cluster.IPsToNodeInfos(cl.workerNodes)
		if err != nil {
			return err
		}

		var allNodeInfos []cluster.NodeInfo
		allNodeInfos = append(allNodeInfos, controlPlaneNodeInfos...)
		allNodeInfos = append(allNodeInfos, workerNodeInfos...)

		cl.nodeInfos = allNodeInfos
		cl.nodeInfosByType = map[machine.Type][]cluster.NodeInfo{
			machine.TypeControlPlane: controlPlaneNodeInfos,
			machine.TypeWorker:       workerNodeInfos,
		}
	}

	return nil
}

func (cl *clusterState) String() string {
	return fmt.Sprintf("control plane: %q, worker: %q", cl.controlPlaneNodes, cl.workerNodes)
}
