// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package runtime provides the runtime implementation.
package runtime

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/cluster/check"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	clusterapi "github.com/siderolabs/talos/pkg/machinery/api/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	clusterres "github.com/siderolabs/talos/pkg/machinery/resources/cluster"
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

	checkCtx, checkCtxCancel := context.WithTimeout(srv.Context(), in.WaitTimeout.AsDuration())
	defer checkCtxCancel()

	md := metadata.New(nil)
	authz.SetMetadata(md, authz.GetRoles(srv.Context()))
	checkCtx = metadata.NewOutgoingContext(checkCtx, md)

	r := s.Controller.Runtime()

	clusterInfo, err := buildClusterInfo(checkCtx, in, r, *k8sProvider)
	if err != nil {
		return err
	}

	state := struct {
		cluster.ClientProvider
		cluster.K8sProvider
		cluster.Info
	}{
		ClientProvider: clientProvider,
		K8sProvider:    k8sProvider,
		Info:           clusterInfo,
	}

	nodeInternalIPs := xslices.Map(clusterInfo.Nodes(), func(info cluster.NodeInfo) string {
		return info.InternalIP.String()
	})

	if err := srv.Send(&clusterapi.HealthCheckProgress{
		Message: fmt.Sprintf("discovered nodes: %q", nodeInternalIPs),
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
	nodeInfos       []cluster.NodeInfo
	nodeInfosByType map[machine.Type][]cluster.NodeInfo
}

func (cl *clusterState) Nodes() []cluster.NodeInfo {
	return cl.nodeInfos
}

func (cl *clusterState) NodesByType(t machine.Type) []cluster.NodeInfo {
	return cl.nodeInfosByType[t]
}

func (cl *clusterState) String() string {
	return fmt.Sprintf("control plane: %q, worker: %q",
		xslices.Map(cl.nodeInfosByType[machine.TypeControlPlane], func(info cluster.NodeInfo) string {
			return info.InternalIP.String()
		}),
		xslices.Map(cl.nodeInfosByType[machine.TypeWorker], func(info cluster.NodeInfo) string {
			return info.InternalIP.String()
		}))
}

//nolint:gocyclo
func buildClusterInfo(ctx context.Context,
	req *clusterapi.HealthCheckRequest,
	r runtime.Runtime,
	cli cluster.KubernetesClient,
) (cluster.Info, error) {
	controlPlaneNodes := req.GetClusterInfo().GetControlPlaneNodes()
	workerNodes := req.GetClusterInfo().GetWorkerNodes()

	// if the node list is explicitly provided, use it
	if len(controlPlaneNodes) != 0 || len(workerNodes) != 0 {
		controlPlaneNodeInfos, err := cluster.IPsToNodeInfos(controlPlaneNodes)
		if err != nil {
			return nil, err
		}

		workerNodeInfos, err := cluster.IPsToNodeInfos(workerNodes)
		if err != nil {
			return nil, err
		}

		return &clusterState{
			nodeInfos: append(slices.Clone(controlPlaneNodeInfos), workerNodeInfos...),
			nodeInfosByType: map[machine.Type][]cluster.NodeInfo{
				machine.TypeControlPlane: controlPlaneNodeInfos,
				machine.TypeWorker:       workerNodeInfos,
			},
		}, nil
	}

	// try to discover nodes using discovery service
	discoveryMemberList, err := getDiscoveryMemberList(ctx, r)
	if err != nil {
		log.Printf("discovery service returned error: %v\n", err)
	}

	// discovery service returned some nodes, use them
	if len(discoveryMemberList) > 0 {
		return check.NewDiscoveredClusterInfo(discoveryMemberList)
	}

	// as the last resort, get the nodes from the cluster itself
	k8sCli, err := cli.K8sClient(ctx)
	if err != nil {
		return nil, err
	}

	nodeList, err := k8sCli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nodeInfos := make([]cluster.NodeInfo, len(nodeList.Items))
	nodeInfosByType := map[machine.Type][]cluster.NodeInfo{}

	for i, node := range nodeList.Items {
		nodeInfo, err2 := k8sNodeToNodeInfo(&node)
		if err2 != nil {
			return nil, err
		}

		if isControlPlaneNode(&node) {
			nodeInfosByType[machine.TypeControlPlane] = append(nodeInfosByType[machine.TypeControlPlane], *nodeInfo)
		} else {
			nodeInfosByType[machine.TypeWorker] = append(nodeInfosByType[machine.TypeWorker], *nodeInfo)
		}

		nodeInfos[i] = *nodeInfo
	}

	return &clusterState{
		nodeInfos:       nodeInfos,
		nodeInfosByType: nodeInfosByType,
	}, nil
}

func k8sNodeToNodeInfo(node *corev1.Node) (*cluster.NodeInfo, error) {
	if node == nil {
		return nil, nil
	}

	var internalIP netip.Addr

	ips := make([]netip.Addr, 0, len(node.Status.Addresses))

	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			ip, err := netip.ParseAddr(address.Address)
			if err != nil {
				return nil, err
			}

			internalIP = ip
			ips = append(ips, ip)
		} else if address.Type == corev1.NodeExternalIP {
			ip, err := netip.ParseAddr(address.Address)
			if err != nil {
				return nil, err
			}

			ips = append(ips, ip)
		}
	}

	return &cluster.NodeInfo{
		InternalIP: internalIP,
		IPs:        ips,
	}, nil
}

func getDiscoveryMemberList(ctx context.Context, runtime runtime.Runtime) ([]*clusterres.Member, error) {
	res := runtime.State().V1Alpha2().Resources()

	list, err := safe.StateListAll[*clusterres.Member](ctx, res)
	if err != nil {
		return nil, err
	}

	return safe.ToSlice(list, func(m *clusterres.Member) *clusterres.Member { return m }), nil
}

func isControlPlaneNode(node *corev1.Node) bool {
	for key := range node.Labels {
		if key == constants.LabelNodeRoleControlPlane {
			return true
		}
	}

	return false
}
