// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api && integration_k8s

package api

import (
	"bytes"
	"context"
	"net/netip"
	"text/template"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	networkres "github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/provision"
)

type bgpWorkloadHelper struct {
	suite *base.K8sSuite

	fabricInstance string
	fabricASN      uint32
	workloadTable  nethelpers.RoutingTable
}

func renderBGPManifests(
	suite *base.K8sSuite,
	name string,
	manifestTemplate []byte,
	data any,
) []unstructured.Unstructured {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(manifestTemplate))
	suite.Require().NoError(err)

	var rendered bytes.Buffer

	suite.Require().NoError(tmpl.Execute(&rendered, data))

	return suite.ParseManifests(rendered.Bytes())
}

func (helper bgpWorkloadHelper) assertTopology(
	nodeCtx context.Context,
	vethName, vethPeerName, vrfName string,
	vethPrefix, vethPeerPrefix netip.Prefix,
) {
	rtestutils.AssertResource(
		nodeCtx,
		helper.suite.T(),
		helper.suite.Client.COSI,
		vethName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal(networkres.LinkKindVeth, link.TypedSpec().Kind)
			asrt.Equal(vethPeerName, link.TypedSpec().Veth.PeerName)
			asrt.Zero(link.TypedSpec().MasterIndex)
			asrt.Empty(link.TypedSpec().SlaveKind)
		},
	)
	rtestutils.AssertResource(
		nodeCtx,
		helper.suite.T(),
		helper.suite.Client.COSI,
		vethPeerName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal(networkres.LinkKindVeth, link.TypedSpec().Kind)
			asrt.Equal(vethName, link.TypedSpec().Veth.PeerName)
			asrt.NotZero(link.TypedSpec().MasterIndex)
			asrt.Equal("vrf", link.TypedSpec().SlaveKind)
		},
	)
	rtestutils.AssertResource(
		nodeCtx,
		helper.suite.T(),
		helper.suite.Client.COSI,
		vrfName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal(networkres.LinkKindVRF, link.TypedSpec().Kind)
			asrt.Equal(helper.workloadTable, link.TypedSpec().VRFMaster.Table)
		},
	)

	rtestutils.AssertResources(
		nodeCtx,
		helper.suite.T(),
		helper.suite.Client.COSI,
		[]resource.ID{
			vethName + "/" + vethPrefix.String(),
			vethPeerName + "/" + vethPeerPrefix.String(),
		},
		func(address *networkres.AddressStatus, asrt *assert.Assertions) {
			asrt.Contains([]string{vethName, vethPeerName}, address.TypedSpec().LinkName)
			asrt.Zero(address.TypedSpec().Flags & nethelpers.AddressFlags(nethelpers.AddressTentative))
			asrt.Zero(address.TypedSpec().Flags & nethelpers.AddressFlags(nethelpers.AddressDADFailed))
		},
	)
}

func (helper bgpWorkloadHelper) waitForPeer(nodeCtx context.Context, id, instance string, peerASN uint32) {
	helper.suite.Require().Eventually(func() bool {
		peer, err := safe.StateGetByID[*networkres.BGPPeerStatus](nodeCtx, helper.suite.Client.COSI, id)
		if err != nil {
			return false
		}

		spec := peer.TypedSpec()

		return spec.Instance == instance && spec.State == nethelpers.BGPSessionStateEstablished && spec.PeerASN == peerASN
	}, 2*time.Minute, time.Second, "BGP peer %q did not reach Established", id)
}

func (helper bgpWorkloadHelper) waitForPeerNotEstablished(nodeCtx context.Context, id string) {
	helper.suite.Require().Eventually(func() bool {
		peer, err := safe.StateGetByID[*networkres.BGPPeerStatus](nodeCtx, helper.suite.Client.COSI, id)
		if err != nil {
			return true
		}

		return peer.TypedSpec().State != nethelpers.BGPSessionStateEstablished
	}, time.Minute, time.Second, "BGP peer %q remained Established", id)
}

func (helper bgpWorkloadHelper) assertNoRoute(nodeCtx context.Context, destination netip.Prefix) {
	helper.assertNoRouteInTable(nodeCtx, destination, helper.workloadTable)
}

func (helper bgpWorkloadHelper) assertNoRouteInTable(
	nodeCtx context.Context,
	destination netip.Prefix,
	table nethelpers.RoutingTable,
) {
	helper.suite.Require().Eventually(func() bool {
		routes, err := safe.StateListAll[*networkres.RouteStatus](nodeCtx, helper.suite.Client.COSI)
		if err != nil {
			return false
		}

		for route := range routes.All() {
			if route.TypedSpec().Destination == destination && route.TypedSpec().Table == table {
				return false
			}
		}

		return true
	}, time.Minute, time.Second, "route %s remained in table %s", destination, table)
}

func (helper bgpWorkloadHelper) assertNoRouteSpecInTable(
	nodeCtx context.Context,
	destination netip.Prefix,
	table nethelpers.RoutingTable,
) {
	helper.suite.Require().Eventually(func() bool {
		routes, err := safe.StateListAll[*networkres.RouteSpec](nodeCtx, helper.suite.Client.COSI)
		if err != nil {
			return false
		}

		for route := range routes.All() {
			spec := route.TypedSpec()
			if spec.Destination == destination && spec.Table == table && spec.Protocol == nethelpers.ProtocolBGP {
				return false
			}
		}

		return true
	}, time.Minute, time.Second, "BGP RouteSpec %s remained in table %s", destination, table)
}

func (helper bgpWorkloadHelper) waitForFabricRoute(nodeCtx context.Context, destination netip.Prefix) {
	helper.suite.Require().Eventually(func() bool {
		routes, err := safe.StateListAll[*networkres.RouteStatus](nodeCtx, helper.suite.Client.COSI)
		if err != nil {
			return false
		}

		for route := range routes.All() {
			spec := route.TypedSpec()
			if spec.Destination != destination || spec.Table != nethelpers.TableMain || spec.Protocol != nethelpers.ProtocolBGP {
				continue
			}

			if len(spec.NextHops) > 0 {
				return true
			}

			return spec.Gateway.IsValid() && !spec.Gateway.IsUnspecified()
		}

		return false
	}, 2*time.Minute, time.Second, "fabric route %s was not installed on the client node", destination)
}

func (helper bgpWorkloadHelper) waitForFabricRouteSpec(nodeCtx context.Context, destination netip.Prefix) {
	helper.suite.Require().Eventually(func() bool {
		routes, err := safe.StateListAll[*networkres.RouteSpec](nodeCtx, helper.suite.Client.COSI)
		if err != nil {
			return false
		}

		for route := range routes.All() {
			spec := route.TypedSpec()
			if spec.Destination == destination && spec.Table == nethelpers.TableMain && spec.Protocol == nethelpers.ProtocolBGP {
				return true
			}
		}

		return false
	}, 2*time.Minute, time.Second, "fabric RouteSpec %s was not published on the client node", destination)
}

func (helper bgpWorkloadHelper) fabricSessions(nodeCtx context.Context) map[resource.ID]time.Time {
	peers, err := safe.StateListAll[*networkres.BGPPeerStatus](nodeCtx, helper.suite.Client.COSI)
	helper.suite.Require().NoError(err)

	result := map[resource.ID]time.Time{}

	for peer := range peers.All() {
		spec := peer.TypedSpec()
		if spec.Instance == helper.fabricInstance &&
			spec.PeerASN == helper.fabricASN &&
			spec.State == nethelpers.BGPSessionStateEstablished {
			result[peer.Metadata().ID()] = spec.Since
		}
	}

	return result
}

func (helper bgpWorkloadHelper) assertFabricSessions(nodeCtx context.Context, expected map[resource.ID]time.Time) {
	helper.suite.Require().Eventually(func() bool {
		for id, since := range expected {
			peer, err := safe.StateGetByID[*networkres.BGPPeerStatus](nodeCtx, helper.suite.Client.COSI, id)
			if err != nil {
				return false
			}

			spec := peer.TypedSpec()
			if spec.Instance != helper.fabricInstance ||
				spec.PeerASN != helper.fabricASN ||
				spec.State != nethelpers.BGPSessionStateEstablished ||
				!spec.Since.Equal(since) {
				return false
			}
		}

		return true
	}, time.Minute, time.Second, "fabric BGP sessions restarted or failed during workload route import")
}

func (helper bgpWorkloadHelper) createBackend(ctx context.Context, nodeName, podName, description string) {
	_, err := helper.suite.Clientset.CoreV1().Pods(corev1.NamespaceDefault).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   podName,
			Labels: map[string]string{"app": podName},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{{
				Name:  "nginx",
				Image: BGPBackendImage,
				Ports: []corev1.ContainerPort{{ContainerPort: 80}},
			}},
		},
	}, metav1.CreateOptions{})
	helper.suite.Require().NoError(err)

	helper.suite.T().Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		if err := helper.suite.Clientset.CoreV1().Pods(corev1.NamespaceDefault).Delete(cleanupCtx, podName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			helper.suite.T().Logf("failed to delete %s backend pod: %v", description, err)
		}
	})

	helper.suite.Require().NoError(helper.suite.WaitForPodToBeRunning(ctx, 2*time.Minute, corev1.NamespaceDefault, podName))
}

func (helper bgpWorkloadHelper) waitForServiceVIP(ctx context.Context, serviceName string, vip netip.Addr) {
	helper.suite.Require().Eventually(func() bool {
		service, err := helper.suite.Clientset.CoreV1().Services(corev1.NamespaceDefault).Get(ctx, serviceName, metav1.GetOptions{})
		if err != nil || len(service.Status.LoadBalancer.Ingress) == 0 {
			return false
		}

		return service.Status.LoadBalancer.Ingress[0].IP == vip.String()
	}, 2*time.Minute, time.Second, "service %q was not assigned VIP %s", serviceName, vip)
}

type bgpFabricHTTPProbe struct {
	suite   *base.K8sSuite
	request provision.HTTPProbeRequest
	name    string
}

func newBGPFabricHTTPProbe(
	suite *base.K8sSuite,
	vip netip.Addr,
	name string,
) *bgpFabricHTTPProbe {
	suite.Require().NotNil(suite.Cluster, "external fabric HTTP probe requires a provisioned cluster")
	suite.Require().NotNil(suite.HTTPProbeProvisioner, "active provisioner does not support external HTTP probes")

	return &bgpFabricHTTPProbe{
		suite: suite,
		request: provision.HTTPProbeRequest{
			IP:      vip,
			Port:    80,
			Path:    "/",
			Timeout: provision.HTTPProbeDefaultTimeout,
		},
		name: name,
	}
}

func (probe *bgpFabricHTTPProbe) assert(ctx context.Context, shouldSucceed bool) {
	if shouldSucceed {
		response, err := probe.suite.HTTPProbeProvisioner.ProbeHTTP(ctx, probe.suite.Cluster, probe.request)
		probe.suite.Require().NoError(err)
		probe.suite.Require().Empty(response.Failure, "external fabric client could not reach %s VIP", probe.name)
		probe.suite.Require().GreaterOrEqual(response.StatusCode, 200)
		probe.suite.Require().Less(response.StatusCode, 300)
		probe.suite.Require().Contains(string(response.Body), "Welcome to nginx")

		return
	}

	probe.suite.Require().Eventually(func() bool {
		response, err := probe.suite.HTTPProbeProvisioner.ProbeHTTP(ctx, probe.suite.Cluster, probe.request)

		return err == nil && response.Failure != ""
	}, 2*time.Minute, 2*time.Second, "external fabric client continued to reach the withdrawn %s VIP", probe.name)
}
