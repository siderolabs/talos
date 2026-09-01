// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"crypto/tls"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/siderolabs/go-retry/retry"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// FirewallSuite ...
type FirewallSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *FirewallSuite) SuiteName() string {
	return "api.FirewallSuite"
}

// SetupTest ...
func (suite *FirewallSuite) SetupTest() {
	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state can't guarantee availability of kubelet IPs")
	}

	// make sure we abort at some point in time, but give enough room for Resets and for the
	// NodePort to become reachable while retrying under network chaos (packet loss/latency).
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)
}

// TearDownTest ...
func (suite *FirewallSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestKubeletAccess verifies that without firewall kubelet API is available, and not available otherwise.
func (suite *FirewallSuite) TestKubeletAccess() {
	allNodes := suite.DiscoverNodeInternalIPs(suite.ctx)

	_, err := safe.StateGetByID[*network.NfTablesChain](client.WithNode(suite.ctx, allNodes[0]), suite.Client.COSI, "ingress")
	firewallEnabled := err == nil

	eg, ctx := errgroup.WithContext(suite.ctx)

	transport := cleanhttp.DefaultTransport()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	client := http.Client{
		Transport: transport,
	}

	for _, node := range allNodes {
		eg.Go(func() error {
			attemptCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(
				attemptCtx,
				http.MethodGet,
				fmt.Sprintf("https://%s/healthz", net.JoinHostPort(node, strconv.Itoa(constants.KubeletPort))),
				nil,
			)
			if err != nil {
				return err
			}

			resp, err := client.Do(req)

			if resp != nil {
				resp.Body.Close() //nolint:errcheck
			}

			if firewallEnabled {
				if err == nil {
					return errors.New("kubelet API should not be available")
				}

				if !errors.Is(err, os.ErrDeadlineExceeded) && !errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("unexpected error: %w", err)
				}
			} else if err != nil {
				return fmt.Errorf("kubelet API should be available: %w", err)
			}

			return nil
		})
	}

	suite.Require().NoError(eg.Wait())
}

//go:embed testdata/nodeport.yaml
var nodePortServiceYAML []byte

// TestNodePortAccess verifies that without firewall NodePort is available, and not available otherwise.
//
//nolint:gocyclo
func (suite *FirewallSuite) TestNodePortAccess() {
	allNodes := suite.DiscoverNodeInternalIPs(suite.ctx)

	chain, err := safe.StateGetByID[*network.NfTablesChain](client.WithNode(suite.ctx, allNodes[0]), suite.Client.COSI, "ingress")
	firewallEnabled := err == nil
	firewallDefaultBlock := firewallEnabled && chain.TypedSpec().Policy == nethelpers.VerdictDrop

	// our blocking only works with kube-proxy, so we need to make sure it's running
	out, err := suite.Clientset.CoreV1().Pods("kube-system").List(suite.ctx, metav1.ListOptions{LabelSelector: "k8s-app=kube-proxy"})
	suite.Require().NoError(err)

	if len(out.Items) == 0 {
		suite.T().Skip("kube-proxy not running")
	}

	// create a deployment with a NodePort service
	localPathStorage := suite.ParseManifests(nodePortServiceYAML)

	suite.T().Cleanup(func() {
		cleanUpCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		suite.DeleteManifests(cleanUpCtx, localPathStorage)
	})

	suite.ApplyManifests(suite.ctx, localPathStorage)

	// A NodePort without a ready endpoint only proves that kube-proxy rejects the
	// connection. Wait for the workload first, especially when the image comes
	// from the image cache or an air-gapped registry.
	suite.Require().NoError(suite.WaitForDeploymentAvailable(suite.ctx, time.Minute, "default", "test-nginx", 1))

	// fetch the NodePort service
	// read back Service to figure out the ports
	svc, err := suite.Clientset.CoreV1().Services("default").Get(suite.ctx, "test-nginx", metav1.GetOptions{})
	suite.Require().NoError(err)

	var nodePort int

	for _, portSpec := range svc.Spec.Ports {
		nodePort = int(portSpec.NodePort)
	}

	suite.Require().NotZero(nodePort)

	// probe dials the NodePort once. In the default-block case a single successful connection
	// is a hard failure; otherwise a not-yet-reachable NodePort is retryable (see below).
	probe := func(ctx context.Context, node string) error {
		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var d net.Dialer

		conn, err := d.DialContext(attemptCtx, "tcp", net.JoinHostPort(node, strconv.Itoa(nodePort)))
		if conn != nil {
			conn.Close() //nolint:errcheck
		}

		if firewallDefaultBlock {
			if err == nil {
				return errors.New("nodePort API should not be available")
			}

			if !errors.Is(err, os.ErrDeadlineExceeded) && !errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("unexpected error: %w", err)
			}

			return nil
		}

		// The NodePort must become reachable, but that takes a moment: the service proxy may not
		// be ready yet (connection refused) and, under network chaos, packets may be dropped or
		// delayed (i/o timeout). Both are transient, so retry rather than failing on first attempt.
		if err != nil {
			return retry.ExpectedError(fmt.Errorf("nodePort API not available yet: %w", err))
		}

		return nil
	}

	eg, ctx := errgroup.WithContext(suite.ctx)

	for _, node := range allNodes {
		eg.Go(func() error {
			return retry.Constant(time.Minute, retry.WithUnits(time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
				return probe(ctx, node)
			})
		})
	}

	if err := eg.Wait(); err != nil {
		suite.describeTestNginx()
		suite.Require().NoError(err)
	}
}

// describeTestNginx dumps pod status, container states and events for the test-nginx
// deployment to help diagnose why the NodePort never became reachable.
func (suite *FirewallSuite) describeTestNginx() {
	dumpCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pods, err := suite.Clientset.CoreV1().Pods("default").List(dumpCtx, metav1.ListOptions{LabelSelector: "app=test-nginx"})
	if err != nil {
		suite.T().Logf("failed to list test-nginx pods: %v", err)

		return
	}

	if len(pods.Items) == 0 {
		suite.T().Log("no test-nginx pods found")
	}

	for _, pod := range pods.Items {
		suite.T().Logf("pod %s: phase=%s node=%s reason=%q message=%q",
			pod.Name, pod.Status.Phase, pod.Spec.NodeName, pod.Status.Reason, pod.Status.Message)

		for _, cs := range pod.Status.ContainerStatuses {
			switch {
			case cs.State.Waiting != nil:
				suite.T().Logf("  container %s waiting: %s: %s", cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message)
			case cs.State.Terminated != nil:
				suite.T().Logf("  container %s terminated: %s: %s", cs.Name, cs.State.Terminated.Reason, cs.State.Terminated.Message)
			case cs.State.Running != nil:
				suite.T().Logf("  container %s running, ready=%t", cs.Name, cs.Ready)
			}
		}

		events, err := suite.Clientset.CoreV1().Events("default").List(dumpCtx, metav1.ListOptions{
			FieldSelector: "involvedObject.name=" + pod.Name,
		})
		if err != nil {
			suite.T().Logf("  failed to list events for pod %s: %v", pod.Name, err)

			continue
		}

		for _, ev := range events.Items {
			suite.T().Logf("  event %s %s: %s", ev.Type, ev.Reason, ev.Message)
		}
	}
}

const (
	// managementLinkAlias is the alias the provisioner gives the management link of every node.
	managementLinkAlias = "net0"

	// probeTargetNamespace is the namespace the probe target pod runs in.
	probeTargetNamespace = "default"
)

// TestLinkIngress verifies that the link ingress filter works as expected.
//
//nolint:gocyclo
func (suite *FirewallSuite) TestLinkIngress() {
	if suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping LinkIngress tests on non qemu provisioner")
	}

	target := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	targetCtx := client.WithNode(suite.ctx, target)

	var prober string

	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		if node != target {
			prober = node

			break
		}
	}

	suite.Require().NotEmpty(prober, "expected at least two nodes")

	targetCfg, err := suite.ReadConfigFromNode(targetCtx)
	suite.Require().NoError(err)

	for _, doc := range targetCfg.Documents() {
		if doc.Kind() == networkcfg.LinkIngressKind {
			suite.T().Skipf("node %q already has link ingress filtering configured", target)
		}
	}

	targetAddr, err := netip.ParseAddr(target)
	suite.Require().NoError(err)

	targetNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, target)
	suite.Require().NoError(err)

	probeTarget, err := suite.NewPod("link-ingress-target")
	suite.Require().NoError(err)

	probeTarget = probeTarget.WithNodeName(targetNode.Name).WithNamespace(probeTargetNamespace)

	suite.Require().NoError(probeTarget.Create(suite.ctx, time.Minute))

	suite.T().Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		probeTarget.Delete(cleanupCtx) //nolint:errcheck
	})

	targetPod, err := suite.Clientset.CoreV1().Pods(probeTargetNamespace).Get(suite.ctx, probeTarget.Name(), metav1.GetOptions{})
	suite.Require().NoError(err)

	probeAddr, err := netip.ParseAddr(targetPod.Status.PodIP)
	suite.Require().NoError(err)

	links, err := safe.ReaderListAll[*network.LinkStatus](targetCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	var linkName string

	for link := range links.All() {
		if link.TypedSpec().Alias == managementLinkAlias {
			linkName = link.Metadata().ID()

			break
		}
	}

	suite.Require().NotEmpty(linkName, "expected to find a link aliased %q", managementLinkAlias)

	suite.T().Logf(
		"filtering link %q on node %q, probing pod %s/%s (%s) from node %q",
		linkName, target, probeTargetNamespace, probeTarget.Name(), probeAddr, prober,
	)

	proberNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, prober)
	suite.Require().NoError(err)

	probePod, err := suite.NewPrivilegedPod("link-ingress-probe")
	suite.Require().NoError(err)

	probePod = probePod.WithNodeName(proberNode.Name).WithQuiet(true)

	suite.Require().NoError(probePod.Create(suite.ctx, time.Minute))

	suite.T().Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		probePod.Delete(cleanupCtx) //nolint:errcheck
	})

	probePrefix := netip.PrefixFrom(probeAddr, probeAddr.BitLen())

	proberLinkCfg := networkcfg.NewLinkConfigV1Alpha1(managementLinkAlias)
	proberLinkCfg.LinkRoutes = []networkcfg.RouteConfig{
		{
			RouteDestination: meta.Prefix{Prefix: probePrefix},
			RouteGateway:     meta.Addr{Addr: targetAddr},
		},
	}

	suite.T().Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		// drop just the route the test added
		suite.PatchMachineConfig(client.WithNode(cleanupCtx, prober), map[string]any{
			"apiVersion": "v1alpha1",
			"kind":       networkcfg.LinkKind,
			"name":       managementLinkAlias,
			"routes": []any{
				map[string]any{
					"destination": probePrefix.String(),
					"$patch":      "delete",
				},
			},
		})
	})

	suite.PatchMachineConfig(client.WithNode(suite.ctx, prober), proberLinkCfg)

	assertProbe := func(expectReachable bool) {
		suite.Require().NoError(retry.Constant(30*time.Second, retry.WithUnits(time.Second)).RetryWithContext(
			suite.ctx,
			func(ctx context.Context) error {
				_, _, err := probePod.Exec(ctx, fmt.Sprintf("ping -c 1 -W 1 %s", probeAddr))

				switch {
				case expectReachable && err != nil:
					return retry.ExpectedErrorf("probe address is not reachable yet: %s", err)
				case !expectReachable && err == nil:
					return retry.ExpectedErrorf("probe address is still reachable")
				}

				return nil
			},
		))
	}

	suite.T().Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		nodeCtx := client.WithNode(cleanupCtx, target)

		suite.RemoveMachineConfigDocumentsByName(nodeCtx, networkcfg.LinkIngressKind, linkName)
	})

	// the pod answers before the filter is turned on
	assertProbe(true)

	linkIngressCfg := networkcfg.NewLinkIngressConfigV1Alpha1(linkName)

	// turn the filter on: the probe address is not one of the node's accepted addresses anymore
	suite.PatchMachineConfig(targetCtx, linkIngressCfg)
	assertProbe(false)

	// allow the probe address: should now pass
	linkIngressCfg.DestinationAddressesConfig = []meta.Prefix{
		{Prefix: netip.PrefixFrom(probeAddr, probeAddr.BitLen())},
		// allow node's own address, otherwise access to the node is lost
		{Prefix: netip.PrefixFrom(targetAddr, targetAddr.BitLen())},
	}

	suite.PatchMachineConfig(targetCtx, linkIngressCfg)
	assertProbe(true)

	// remove link ingress, should still pass
	suite.RemoveMachineConfigDocumentsByName(targetCtx, networkcfg.LinkIngressKind, linkName)
	assertProbe(true)
}

func init() {
	allSuites = append(allSuites, new(FirewallSuite))
}
