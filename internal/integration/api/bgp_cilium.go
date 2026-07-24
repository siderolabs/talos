// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api && integration_k8s

package api

import (
	"context"
	_ "embed"
	"net/netip"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	networkres "github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const (
	ciliumVethName     = "veth-cilium"
	ciliumVethPeerName = "veth-crouter"
	ciliumWorkloadVRF  = "vrf-cilium"
	ciliumWorkloadBGP  = "cilium"
	ciliumFabricName   = "fabric"

	ciliumTalosASN   uint32 = 4_200_000_002
	ciliumSpeakerASN uint32 = 4_200_000_003
	ciliumFabricASN  uint32 = 65_000

	ciliumBGPInstanceName = "talos-local"
	ciliumBGPPeerName     = "talos-vrf"
	ciliumPeerConfigName  = "talos-vrf"
	ciliumAdvertisement   = "talos-workload-routes"
	ciliumPoolName        = "talos-bgp-test"
	ciliumBackendName     = "cilium-bgp-backend"
	ciliumServiceName     = "cilium-bgp-service"
	ciliumNamespace       = "kube-system"

	ciliumAdvertisementLabel = "bgp.talos.dev/advertisement"
	ciliumServiceLabel       = "bgp.talos.dev/service"
	ciliumTestLabelValue     = "cilium-fabric"
)

const ciliumWorkloadVRFTable = nethelpers.RoutingTable(89)

var (
	ciliumSpeakerPrefix = netip.MustParsePrefix("fda1:b2c3:d4e5:2::/127")
	ciliumRouterPrefix  = netip.MustParsePrefix("fda1:b2c3:d4e5:2::1/127")
	ciliumVIPPrefix     = netip.MustParsePrefix("203.0.113.100/32")
	ciliumImportPrefix  = netip.MustParsePrefix("203.0.113.0/24")
)

//go:embed testdata/bgp-cilium.yaml
var ciliumManifestsYAML []byte

type ciliumManifestData struct {
	PoolName           string
	AdvertisementName  string
	PeerConfigName     string
	InstanceName       string
	PeerName           string
	NodeName           string
	VIPPrefix          string
	RouterAddress      string
	SourceInterface    string
	ServiceLabel       string
	AdvertisementLabel string
	LabelValue         string
	TalosASN           uint32
	SpeakerASN         uint32
}

// BGPCiliumSuite verifies that the real Cilium BGP Control Plane can feed service and PodCIDR
// routes into a RIB-only Talos workload instance for one-way redistribution through the CLOS fabric.
type BGPCiliumSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName implements base.NamedSuite.
func (suite *BGPCiliumSuite) SuiteName() string {
	return "api.BGPCiliumSuite"
}

// SetupTest initializes the test timeout.
func (suite *BGPCiliumSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 15*time.Minute)
}

// TearDownTest cancels the test context.
func (suite *BGPCiliumSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestCiliumVRFBGP verifies Cilium Service and PodCIDR advertisements across the full CLOS fabric.
func (suite *BGPCiliumSuite) TestCiliumVRFBGP() { //nolint:gocyclo
	if !suite.BGPCLOSEnabled || !suite.CiliumBGPEnabled {
		suite.T().Skip("skipping Cilium fabric test; enable with -talos.bgp.clos -talos.bgp.cilium on a Cilium CNI cluster created with --with-bgp-clos")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping Cilium BGP test since provisioner is not qemu")
	}

	suite.Mapper.Reset()

	_, err := suite.Mapper.RESTMapping(schema.GroupKind{
		Group: "cilium.io",
		Kind:  "CiliumBGPClusterConfig",
	}, "v2")
	suite.Require().NoError(err, "Cilium BGP Control Plane v2 is not installed")
	suite.assertStrictKubeProxyReplacement()

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	controlPlanes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)
	suite.Require().NotEmpty(controlPlanes, "no control-plane node available as a fabric route witness")

	clientNode := controlPlanes[0]
	clientNodeCtx := client.WithNode(suite.ctx, clientNode)

	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	podCIDR, err := netip.ParsePrefix(k8sNode.Spec.PodCIDR)
	suite.Require().NoError(err, "worker has no valid Kubernetes PodCIDR")

	httpProbe := newBGPFabricHTTPProbe(
		&suite.K8sSuite,
		ciliumVIPPrefix.Addr(),
		"Cilium",
	)

	workloadHelper := bgpWorkloadHelper{
		suite:          &suite.K8sSuite,
		fabricInstance: ciliumFabricName,
		fabricASN:      ciliumFabricASN,
		workloadTable:  ciliumWorkloadVRFTable,
	}

	fabricPatch := network.NewBGPInstanceConfigV1Alpha1(ciliumFabricName)

	fabricSessions := workloadHelper.fabricSessions(nodeCtx)
	suite.Require().NotEmpty(fabricSessions, "worker has no established fabric sessions")

	suite.T().Logf(
		"testing Cilium BGP service and PodCIDR %s from worker %q (%s) through VRF table %s and the CLOS fabric to client node %s",
		podCIDR,
		k8sNode.Name,
		node,
		ciliumWorkloadVRFTable,
		clientNode,
	)

	veth := network.NewVethConfigV1Alpha1(ciliumVethName, ciliumVethPeerName)
	veth.LinkUp = new(true)
	veth.LinkAddresses = []network.AddressConfig{{AddressAddress: ciliumSpeakerPrefix}}
	veth.VethPeer.LinkUp = new(true)
	veth.VethPeer.LinkAddresses = []network.AddressConfig{{AddressAddress: ciliumRouterPrefix}}

	vrf := network.NewVRFConfigV1Alpha1(ciliumWorkloadVRF)
	vrf.VRFLinks = []string{ciliumVethPeerName}
	vrf.VRFTable = ciliumWorkloadVRFTable
	vrf.LinkUp = new(true)

	workloadBGP := network.NewBGPInstanceConfigV1Alpha1(ciliumWorkloadBGP)
	workloadBGP.BGPVRF = ciliumWorkloadVRF
	workloadBGP.BGPLocalASN = ciliumTalosASN
	workloadBGP.BGPRouterID = meta.Addr{Addr: netip.MustParseAddr(node)}
	workloadBGP.BGPInstallRoutes = new(false)
	workloadBGP.BGPNeighborConfigs = []network.BGPNeighborConfig{{
		NeighborAddressConfig: meta.Addr{Addr: ciliumSpeakerPrefix.Addr()},
		NeighborPeerASN:       ciliumSpeakerASN,
		NeighborPassive:       true,
		NeighborHoldTime:      9 * time.Second,
	}}

	fabricPatch.BGPImportRoutes = append(fabricPatch.BGPImportRoutes, network.BGPImportRoute{
		ImportBGPInstance: ciliumWorkloadBGP,
		ImportPrefixes: []meta.Prefix{
			{Prefix: ciliumImportPrefix},
			{Prefix: podCIDR},
		},
	})

	suite.PatchMachineConfig(
		nodeCtx,
		veth,
		vrf,
		workloadBGP,
		fabricPatch,
	)

	workloadHelper.assertTopology(
		nodeCtx,
		ciliumVethName,
		ciliumVethPeerName,
		ciliumWorkloadVRF,
		ciliumSpeakerPrefix,
		ciliumRouterPrefix,
	)
	workloadHelper.assertFabricSessions(nodeCtx, fabricSessions)

	suite.ciliumCaptureLogsOnFailure(k8sNode.Name)

	ciliumObjects := suite.ciliumBGPObjects(k8sNode.Name)
	suite.ApplyManifests(suite.ctx, ciliumObjects)
	suite.T().Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.DeleteManifests(cleanupCtx, ciliumObjects)
	})

	peerID := ciliumWorkloadBGP + "/" + ciliumSpeakerPrefix.Addr().String()
	workloadHelper.waitForPeer(nodeCtx, peerID, ciliumWorkloadBGP, ciliumSpeakerASN)

	workloadHelper.createBackend(suite.ctx, k8sNode.Name, ciliumBackendName, "Cilium")
	suite.ciliumCreateService()
	workloadHelper.waitForServiceVIP(suite.ctx, ciliumServiceName, ciliumVIPPrefix.Addr())

	workloadHelper.waitForFabricRoute(clientNodeCtx, ciliumVIPPrefix)
	// Cilium already owns the client node's kernel route to this PodCIDR. Assert Talos's desired
	// BGP RouteSpec directly so the pre-existing CNI route cannot hide fabric propagation.
	workloadHelper.waitForFabricRouteSpec(clientNodeCtx, podCIDR)
	workloadHelper.assertNoRoute(nodeCtx, ciliumVIPPrefix)
	workloadHelper.assertNoRouteInTable(nodeCtx, ciliumVIPPrefix, nethelpers.TableMain)
	workloadHelper.assertNoRouteSpecInTable(nodeCtx, podCIDR, ciliumWorkloadVRFTable)
	workloadHelper.assertNoRouteSpecInTable(nodeCtx, podCIDR, nethelpers.TableMain)
	httpProbe.assert(suite.ctx, true)
	workloadHelper.assertFabricSessions(nodeCtx, fabricSessions)

	suite.DeleteManifests(suite.ctx, ciliumObjects[1:2])
	workloadHelper.assertNoRouteInTable(clientNodeCtx, ciliumVIPPrefix, nethelpers.TableMain)
	workloadHelper.assertNoRouteSpecInTable(clientNodeCtx, podCIDR, nethelpers.TableMain)
	httpProbe.assert(suite.ctx, false)
	workloadHelper.waitForPeer(nodeCtx, peerID, ciliumWorkloadBGP, ciliumSpeakerASN)

	suite.DeleteManifests(suite.ctx, ciliumObjects[3:4])
	workloadHelper.waitForPeerNotEstablished(nodeCtx, peerID)
	workloadHelper.assertFabricSessions(nodeCtx, fabricSessions)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.BGPInstanceKind, ciliumWorkloadBGP)
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.VRFKind, ciliumWorkloadVRF)
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.VethKind, ciliumVethName)

	bgpImportsRoutesDeletePatch := map[string]any{
		"apiVersion": "v1alpha1",
		"kind":       "BGPInstanceConfig",
		"name":       ciliumFabricName,
		"importRoutes": map[string]any{
			"$patch": "delete",
		},
	}
	suite.PatchMachineConfig(nodeCtx, bgpImportsRoutesDeletePatch)

	rtestutils.AssertNoResource[*networkres.BGPPeerStatus](nodeCtx, suite.T(), suite.Client.COSI, peerID)
	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, ciliumVethName)
	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, ciliumVethPeerName)
	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, ciliumWorkloadVRF)
	workloadHelper.assertFabricSessions(nodeCtx, fabricSessions)
}

func (suite *BGPCiliumSuite) assertStrictKubeProxyReplacement() {
	_, err := suite.Clientset.AppsV1().DaemonSets(ciliumNamespace).Get(
		suite.ctx,
		"kube-proxy",
		metav1.GetOptions{},
	)
	suite.Require().True(apierrors.IsNotFound(err), "kube-proxy DaemonSet must be absent in strict Cilium mode: %v", err)

	config, err := suite.Clientset.CoreV1().ConfigMaps(ciliumNamespace).Get(
		suite.ctx,
		"cilium-config",
		metav1.GetOptions{},
	)
	suite.Require().NoError(err)
	suite.Require().Equal("true", config.Data["kube-proxy-replacement"], "Cilium kube-proxy replacement is not enabled")
}

func (suite *BGPCiliumSuite) ciliumBGPObjects(nodeName string) []unstructured.Unstructured {
	objects := renderBGPManifests(&suite.K8sSuite, "bgp-cilium", ciliumManifestsYAML, ciliumManifestData{
		PoolName:           ciliumPoolName,
		AdvertisementName:  ciliumAdvertisement,
		PeerConfigName:     ciliumPeerConfigName,
		InstanceName:       ciliumBGPInstanceName,
		PeerName:           ciliumBGPPeerName,
		NodeName:           nodeName,
		VIPPrefix:          ciliumVIPPrefix.String(),
		RouterAddress:      ciliumRouterPrefix.Addr().String(),
		SourceInterface:    ciliumVethName,
		ServiceLabel:       ciliumServiceLabel,
		AdvertisementLabel: ciliumAdvertisementLabel,
		LabelValue:         ciliumTestLabelValue,
		TalosASN:           ciliumTalosASN,
		SpeakerASN:         ciliumSpeakerASN,
	})

	suite.Require().Len(objects, 4)
	suite.Require().Equal("CiliumBGPAdvertisement", objects[1].GetKind())
	suite.Require().Equal("CiliumBGPClusterConfig", objects[3].GetKind())

	return objects
}

func (suite *BGPCiliumSuite) ciliumCreateService() {
	_, err := suite.Clientset.CoreV1().Services(corev1.NamespaceDefault).Create(suite.ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: ciliumServiceName,
			Labels: map[string]string{
				ciliumServiceLabel: ciliumTestLabelValue,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector:              map[string]string{"app": ciliumBackendName},
			Type:                  corev1.ServiceTypeLoadBalancer,
			LoadBalancerIP:        ciliumVIPPrefix.Addr().String(),
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       80,
				TargetPort: intstr.FromInt32(80),
			}},
		},
	}, metav1.CreateOptions{})
	suite.Require().NoError(err)

	suite.T().Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		if err := suite.Clientset.CoreV1().Services(corev1.NamespaceDefault).Delete(cleanupCtx, ciliumServiceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			suite.T().Logf("failed to delete Cilium service: %v", err)
		}
	})
}

func (suite *BGPCiliumSuite) ciliumCaptureLogsOnFailure(nodeName string) {
	suite.T().Cleanup(func() {
		if !suite.T().Failed() {
			return
		}

		logCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		pods, err := suite.Clientset.CoreV1().Pods(ciliumNamespace).List(logCtx, metav1.ListOptions{
			LabelSelector: "k8s-app=cilium",
		})
		if err != nil {
			suite.T().Logf("failed to list Cilium pods: %v", err)

			return
		}

		for _, pod := range pods.Items {
			if pod.Spec.NodeName == nodeName {
				suite.LogPodLogs(logCtx, ciliumNamespace, pod.Name)
			}
		}
	})
}

func init() {
	allSuites = append(allSuites, new(BGPCiliumSuite))
}
