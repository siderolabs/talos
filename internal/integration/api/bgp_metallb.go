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
	metalLBNamespace = "metallb-system"

	metalLBVethName       = "veth-metallb"
	metalLBVethPeerName   = "veth-router"
	metalLBWorkloadVRF    = "vrf-metallb"
	metalLBWorkloadBGP    = "metallb"
	metalLBFabricName     = "fabric"
	metalLBExtendedNHName = "metallb-extended-nexthop"

	metalLBTalosASN   uint32 = 4_200_000_000
	metalLBSpeakerASN uint32 = 4_200_000_001
	metalLBFabricASN  uint32 = 65_000

	metalLBBackendName = "metallb-bgp-backend"
	metalLBServiceName = "metallb-bgp-service"
	metalLBPoolName    = "talos-bgp-test"
	metalLBPeerName    = "talos-vrf"
	metalLBAdvName     = "talos-bgp-test"
)

const (
	metalLBWorkloadVRFTable = nethelpers.RoutingTable(88)
)

var (
	metalLBRouterPrefix  = netip.MustParsePrefix("fda1:b2c3:d4e5:1::/127")
	metalLBSpeakerPrefix = netip.MustParsePrefix("fda1:b2c3:d4e5:1::1/127")
	metalLBVIPPrefix     = netip.MustParsePrefix("198.51.100.100/32")
	metalLBImportPrefix  = netip.MustParsePrefix("198.51.100.0/24")
)

//go:embed testdata/bgp-metallb.yaml
var metalLBManifestsYAML []byte

type metalLBManifestData struct {
	Namespace           string
	PoolName            string
	PeerName            string
	AdvertisementName   string
	ExtendedNextHopName string
	NodeName            string
	VIPPrefix           string
	RouterAddress       string
	SpeakerAddress      string
	TalosASN            uint32
	SpeakerASN          uint32
}

// BGPMetalLBSuite verifies a host-networked workload speaker through a VRF-bound Talos BGP instance
// and an attribute-preserving route import into the physical fabric instance.
type BGPMetalLBSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName implements base.NamedSuite.
func (suite *BGPMetalLBSuite) SuiteName() string {
	return "api.BGPMetalLBSuite"
}

// SetupTest initializes the test timeout.
func (suite *BGPMetalLBSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 15*time.Minute)
}

// TearDownTest cancels the test context.
func (suite *BGPMetalLBSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestMetalLBVRFBGP verifies a MetalLB service advertisement across the full CLOS fabric.
func (suite *BGPMetalLBSuite) TestMetalLBVRFBGP() { //nolint:gocyclo
	if !suite.BGPCLOSEnabled {
		suite.T().Skip("skipping MetalLB fabric test; enable with -talos.bgp.clos (requires a cluster created with --with-bgp-clos)")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping MetalLB BGP test since provisioner is not qemu")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	controlPlanes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)
	suite.Require().NotEmpty(controlPlanes, "no control-plane node available as a fabric route witness")

	clientNode := controlPlanes[0]
	clientNodeCtx := client.WithNode(suite.ctx, clientNode)

	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	httpProbe := newBGPFabricHTTPProbe(
		&suite.K8sSuite,
		metalLBVIPPrefix.Addr(),
		"MetalLB",
	)

	workloadHelper := bgpWorkloadHelper{
		suite:          &suite.K8sSuite,
		fabricInstance: metalLBFabricName,
		fabricASN:      metalLBFabricASN,
		workloadTable:  metalLBWorkloadVRFTable,
	}

	fabricSessions := workloadHelper.fabricSessions(nodeCtx)
	suite.Require().NotEmpty(fabricSessions, "worker has no established fabric sessions")

	suite.T().Logf(
		"testing MetalLB BGP service from worker %q (%s) through VRF table %s and the CLOS fabric to client node %s",
		k8sNode.Name,
		node,
		metalLBWorkloadVRFTable,
		clientNode,
	)

	veth := network.NewVethConfigV1Alpha1(metalLBVethName, metalLBVethPeerName)
	veth.LinkUp = new(true)
	veth.LinkAddresses = []network.AddressConfig{{AddressAddress: metalLBSpeakerPrefix}}
	veth.VethPeer.LinkUp = new(true)
	veth.VethPeer.LinkAddresses = []network.AddressConfig{{AddressAddress: metalLBRouterPrefix}}

	vrf := network.NewVRFConfigV1Alpha1(metalLBWorkloadVRF)
	vrf.VRFLinks = []string{metalLBVethPeerName}
	vrf.VRFTable = metalLBWorkloadVRFTable
	vrf.LinkUp = new(true)

	workloadBGP := network.NewBGPInstanceConfigV1Alpha1(metalLBWorkloadBGP)
	workloadBGP.BGPVRF = metalLBWorkloadVRF
	workloadBGP.BGPLocalASN = metalLBTalosASN
	workloadBGP.BGPRouterID = meta.Addr{Addr: netip.MustParseAddr(node)}
	workloadBGP.BGPInstallRoutes = new(false)
	workloadBGP.BGPNeighborConfigs = []network.BGPNeighborConfig{{
		NeighborAddressConfig: meta.Addr{Addr: metalLBSpeakerPrefix.Addr()},
		NeighborPeerASN:       metalLBSpeakerASN,
		NeighborHoldTime:      9 * time.Second,
	}}

	fabricPatch := network.NewBGPInstanceConfigV1Alpha1(metalLBFabricName)

	fabricPatch.BGPImportRoutes = []network.BGPImportRoute{{
		ImportBGPInstance: metalLBWorkloadBGP,
		ImportPrefixes:    []meta.Prefix{{Prefix: metalLBImportPrefix}},
	}}

	suite.PatchMachineConfig(
		nodeCtx,
		veth,
		vrf,
		workloadBGP,
		fabricPatch,
	)

	workloadHelper.assertTopology(
		nodeCtx,
		metalLBVethName,
		metalLBVethPeerName,
		metalLBWorkloadVRF,
		metalLBSpeakerPrefix,
		metalLBRouterPrefix,
	)
	workloadHelper.assertFabricSessions(nodeCtx, fabricSessions)

	suite.metalLBInstall()
	suite.Mapper.Reset()
	suite.metalLBAssertFRRK8s(k8sNode.Name)

	metalLBObjects := suite.metalLBObjects(k8sNode.Name)
	suite.ApplyManifests(suite.ctx, metalLBObjects)
	suite.T().Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.DeleteManifests(cleanupCtx, metalLBObjects)
	})

	workloadHelper.waitForPeer(
		nodeCtx,
		metalLBWorkloadBGP+"/"+metalLBSpeakerPrefix.Addr().String(),
		metalLBWorkloadBGP,
		metalLBSpeakerASN,
	)

	workloadHelper.createBackend(suite.ctx, k8sNode.Name, metalLBBackendName, "MetalLB")
	suite.metalLBCreateService()
	workloadHelper.waitForServiceVIP(suite.ctx, metalLBServiceName, metalLBVIPPrefix.Addr())
	workloadHelper.waitForFabricRoute(clientNodeCtx, metalLBVIPPrefix)
	workloadHelper.assertNoRoute(nodeCtx, metalLBVIPPrefix)
	workloadHelper.assertNoRouteInTable(nodeCtx, metalLBVIPPrefix, nethelpers.TableMain)
	httpProbe.assert(suite.ctx, true)
	workloadHelper.assertFabricSessions(nodeCtx, fabricSessions)

	advertisement := metalLBObjects[2:3]
	suite.DeleteManifests(suite.ctx, advertisement)
	workloadHelper.assertNoRoute(nodeCtx, metalLBVIPPrefix)
	workloadHelper.assertNoRouteInTable(clientNodeCtx, metalLBVIPPrefix, nethelpers.TableMain)
	httpProbe.assert(suite.ctx, false)
	workloadHelper.waitForPeer(
		nodeCtx,
		metalLBWorkloadBGP+"/"+metalLBSpeakerPrefix.Addr().String(),
		metalLBWorkloadBGP,
		metalLBSpeakerASN,
	)

	peer := metalLBObjects[1:2]
	suite.DeleteManifests(suite.ctx, peer)
	workloadHelper.waitForPeerNotEstablished(
		nodeCtx,
		metalLBWorkloadBGP+"/"+metalLBSpeakerPrefix.Addr().String(),
	)

	workloadHelper.assertFabricSessions(nodeCtx, fabricSessions)

	bgpImportsRoutesDeletePatch := map[string]any{
		"apiVersion": "v1alpha1",
		"kind":       "BGPInstanceConfig",
		"name":       metalLBFabricName,
		"importRoutes": map[string]any{
			"$patch": "delete",
		},
	}
	suite.PatchMachineConfig(nodeCtx, bgpImportsRoutesDeletePatch)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.BGPInstanceKind, metalLBWorkloadBGP)
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.VRFKind, metalLBWorkloadVRF)
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.VethKind, metalLBVethName)

	rtestutils.AssertNoResource[*networkres.BGPPeerStatus](
		nodeCtx,
		suite.T(),
		suite.Client.COSI,
		metalLBWorkloadBGP+"/"+metalLBSpeakerPrefix.Addr().String(),
	)
	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, metalLBVethName)
	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, metalLBVethPeerName)
	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, metalLBWorkloadVRF)

	workloadHelper.assertFabricSessions(nodeCtx, fabricSessions)
}

func (suite *BGPMetalLBSuite) metalLBInstall() {
	_, err := suite.Clientset.CoreV1().Namespaces().Create(suite.ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: metalLBNamespace,
			Labels: map[string]string{
				"pod-security.kubernetes.io/enforce": "privileged",
			},
		},
	}, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		err = nil
	}

	suite.Require().NoError(err)

	suite.T().Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		if err := suite.Clientset.CoreV1().Namespaces().Delete(cleanupCtx, metalLBNamespace, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			suite.T().Logf("failed to delete namespace %q: %v", metalLBNamespace, err)
		}
	})

	values := []byte(`frrk8s:
  enabled: true
speaker:
  frr:
    enabled: false
`)

	suite.Require().NoError(suite.HelmInstall(
		suite.ctx,
		metalLBNamespace,
		"https://metallb.github.io/metallb",
		MetalLBChartVersion,
		"metallb",
		"metallb",
		values,
	))

	suite.T().Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		if err := suite.HelmUninstall(cleanupCtx, metalLBNamespace, "metallb"); err != nil {
			suite.T().Logf("failed to uninstall MetalLB: %v", err)
		}
	})

	suite.Require().NoError(suite.WaitForDeploymentAvailable(suite.ctx, 3*time.Minute, metalLBNamespace, "metallb-controller", 1))
	suite.Require().NoError(suite.WaitForDaemonSetReady(
		suite.ctx,
		3*time.Minute,
		metalLBNamespace,
		"app.kubernetes.io/component=speaker",
	))
	suite.Require().NoError(suite.WaitForDaemonSetReady(
		suite.ctx,
		3*time.Minute,
		metalLBNamespace,
		"app.kubernetes.io/name=frr-k8s,app.kubernetes.io/component=frr-k8s",
	))

	suite.T().Cleanup(func() {
		if !suite.T().Failed() {
			return
		}

		logCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		for _, selector := range []string{
			"app.kubernetes.io/component=speaker",
			"app.kubernetes.io/name=frr-k8s,app.kubernetes.io/component=frr-k8s",
		} {
			pods, listErr := suite.Clientset.CoreV1().Pods(metalLBNamespace).List(logCtx, metav1.ListOptions{
				LabelSelector: selector,
			})
			if listErr != nil {
				suite.T().Logf("failed to list MetalLB pods matching %q: %v", selector, listErr)

				continue
			}

			for _, pod := range pods.Items {
				suite.LogPodLogs(logCtx, metalLBNamespace, pod.Name)
			}
		}
	})
}

func (suite *BGPMetalLBSuite) metalLBAssertFRRK8s(nodeName string) {
	pods, err := suite.Clientset.CoreV1().Pods(metalLBNamespace).List(suite.ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=speaker",
	})
	suite.Require().NoError(err)

	found := false

	for _, pod := range pods.Items {
		if pod.Spec.NodeName != nodeName {
			continue
		}

		found = true

		suite.Require().Len(pod.Spec.Containers, 1, "MetalLB speaker should delegate BGP to the separate FRR-k8s pod")
		suite.Require().Equal("speaker", pod.Spec.Containers[0].Name)
	}

	suite.Require().True(found, "no MetalLB speaker pod found on node %q", nodeName)

	frrPods, err := suite.Clientset.CoreV1().Pods(metalLBNamespace).List(suite.ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=frr-k8s,app.kubernetes.io/component=frr-k8s",
	})
	suite.Require().NoError(err)

	found = false

	for _, pod := range frrPods.Items {
		if pod.Spec.NodeName == nodeName {
			found = true

			break
		}
	}

	suite.Require().True(found, "no FRR-k8s pod found on node %q", nodeName)
}

func (suite *BGPMetalLBSuite) metalLBObjects(nodeName string) []unstructured.Unstructured {
	objects := renderBGPManifests(&suite.K8sSuite, "bgp-metallb", metalLBManifestsYAML, metalLBManifestData{
		Namespace:           metalLBNamespace,
		PoolName:            metalLBPoolName,
		PeerName:            metalLBPeerName,
		AdvertisementName:   metalLBAdvName,
		ExtendedNextHopName: metalLBExtendedNHName,
		NodeName:            nodeName,
		VIPPrefix:           metalLBVIPPrefix.String(),
		RouterAddress:       metalLBRouterPrefix.Addr().String(),
		SpeakerAddress:      metalLBSpeakerPrefix.Addr().String(),
		TalosASN:            metalLBTalosASN,
		SpeakerASN:          metalLBSpeakerASN,
	})

	suite.Require().Len(objects, 4)
	suite.Require().Equal("BGPPeer", objects[1].GetKind())
	suite.Require().Equal("BGPAdvertisement", objects[2].GetKind())

	return objects
}

func (suite *BGPMetalLBSuite) metalLBCreateService() {
	_, err := suite.Clientset.CoreV1().Services(corev1.NamespaceDefault).Create(suite.ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: metalLBServiceName,
			Annotations: map[string]string{
				"metallb.io/address-pool": metalLBPoolName,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector:              map[string]string{"app": metalLBBackendName},
			Type:                  corev1.ServiceTypeLoadBalancer,
			LoadBalancerIP:        metalLBVIPPrefix.Addr().String(),
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

		if err := suite.Clientset.CoreV1().Services(corev1.NamespaceDefault).Delete(cleanupCtx, metalLBServiceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			suite.T().Logf("failed to delete MetalLB service: %v", err)
		}
	})
}

func init() {
	allSuites = append(allSuites, new(BGPMetalLBSuite))
}
