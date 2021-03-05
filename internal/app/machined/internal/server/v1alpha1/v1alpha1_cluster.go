// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/cluster/check"
	"github.com/talos-systems/talos/pkg/conditions"
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
	clusterState := clusterState{
		controlPlaneNodes: in.GetClusterInfo().GetControlPlaneNodes(),
		workerNodes:       in.GetClusterInfo().GetWorkerNodes(),
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
}

func (cluster *clusterState) Nodes() []string {
	return append([]string(nil), append(cluster.controlPlaneNodes, cluster.workerNodes...)...)
}

func (cluster *clusterState) NodesByType(t machine.Type) []string {
	switch t {
	case machine.TypeInit:
		return nil
	case machine.TypeControlPlane:
		return append([]string(nil), cluster.controlPlaneNodes...)
	case machine.TypeJoin:
		return append([]string(nil), cluster.workerNodes...)
	case machine.TypeUnknown:
		return nil
	default:
		panic("unsupported machine type")
	}
}

func (cluster *clusterState) resolve(ctx context.Context, k8sProvider *cluster.KubernetesClient) error {
	if len(cluster.controlPlaneNodes) == 0 && len(cluster.workerNodes) == 0 {
		var err error

		if _, err = k8sProvider.K8sClient(ctx); err != nil {
			return err
		}

		if cluster.controlPlaneNodes, err = k8sProvider.KubeHelper.NodeIPs(ctx, machine.TypeControlPlane); err != nil {
			return err
		}

		if cluster.workerNodes, err = k8sProvider.KubeHelper.NodeIPs(ctx, machine.TypeJoin); err != nil {
			return err
		}
	}

	return nil
}

func (cluster *clusterState) String() string {
	return fmt.Sprintf("control plane: %q, worker: %q", cluster.controlPlaneNodes, cluster.workerNodes)
}
