// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	criconfig "github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// CommonSuite verifies some default settings such as ulimits.
type CommonSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *CommonSuite) SuiteName() string {
	return "api.CommonSuite"
}

// SetupTest ...
func (suite *CommonSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 15*time.Minute)
}

// TearDownTest ...
func (suite *CommonSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestVirtioModulesLoaded verifies that the virtio modules are loaded.
func (suite *CommonSuite) TestVirtioModulesLoaded() {
	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping virtio test since provisioner is not qemu")
	}

	expectedVirtIOModules := []string{
		"virtio_balloon",
		"virtio_pci",
		"virtio_pci_legacy_dev",
		"virtio_pci_modern_dev",
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	suite.AssertExpectedModules(suite.ctx, node, expectedVirtIOModules)
}

// TestCommonDefaults verifies that the default ulimits are set.
func (suite *CommonSuite) TestCommonDefaults() {
	if suite.Cluster != nil && suite.Cluster.Provisioner() == base.ProvisionerDocker {
		suite.T().Skip("skipping ulimits test since provisioner is docker")
	}

	expectedUlimit := `
core file size (blocks)         (-c) 0
data seg size (kb)              (-d) unlimited
scheduling priority             (-e) 0
file size (blocks)              (-f) unlimited
max locked memory (kb)          (-l) 8192
max memory size (kb)            (-m) unlimited
open files                      (-n) 1048576
POSIX message queues (bytes)    (-q) 819200
real-time priority              (-r) 0
stack size (kb)                 (-s) 8192
cpu time (seconds)              (-t) unlimited
virtual memory (kb)             (-v) unlimited
file locks                      (-x) unlimited
`

	defaultsTestPodDef, err := suite.NewPod("defaults-ulimits-test")
	suite.Require().NoError(err)

	suite.Require().NoError(defaultsTestPodDef.Create(suite.ctx, 5*time.Minute))

	defer defaultsTestPodDef.Delete(suite.ctx) //nolint:errcheck

	stdout, stderr, err := defaultsTestPodDef.Exec(
		suite.ctx,
		"ulimit -c -d -e -f -l -m -n -q -r -s -t -v -x",
	)
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)
	suite.Require().Equal(strings.TrimPrefix(expectedUlimit, "\n"), stdout)
}

// TestDNSResolver verifies that external DNS resolving works from a pod.
func (suite *CommonSuite) TestDNSResolver() {
	if suite.Airgapped {
		suite.T().Skip("skipping test in airgapped mode")
	}

	if suite.Cluster != nil {
		// cluster should be healthy for kube-dns resolving to work
		suite.AssertClusterHealthy(suite.ctx)
	}

	dnsTestPodDef, err := suite.NewPod("dns-test")
	suite.Require().NoError(err)

	suite.Require().NoError(dnsTestPodDef.Create(suite.ctx, 5*time.Minute))

	defer dnsTestPodDef.Delete(suite.ctx) //nolint:errcheck

	stdout, stderr, err := dnsTestPodDef.Exec(
		suite.ctx,
		"wget -S https://www.google.com/",
	)
	suite.Assert().NoError(err)

	suite.Assert().Equal("", stdout)
	suite.Assert().Contains(stderr, "'index.html' saved")

	if suite.T().Failed() {
		suite.LogPodLogsByLabel(suite.ctx, "kube-system", "k8s-app", "kube-dns")

		for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
			suite.DumpLogs(suite.ctx, node, "dns-resolve-cache", "google")
		}

		suite.T().FailNow()
	}

	_, stderr, err = dnsTestPodDef.Exec(
		suite.ctx,
		"apk add --update bind-tools",
	)

	suite.Assert().NoError(err)
	suite.Assert().Empty(stderr, "stderr: %s", stderr)

	if suite.T().Failed() {
		suite.T().FailNow()
	}

	stdout, stderr, err = dnsTestPodDef.Exec(
		suite.ctx,
		"dig really-long-record.dev.siderolabs.io",
	)

	suite.Assert().NoError(err)
	suite.Assert().Contains(stdout, "status: NOERROR")
	suite.Assert().Contains(stdout, "ANSWER: 34")
	suite.Assert().NotContains(stdout, "status: NXDOMAIN")
	suite.Assert().Equal(stderr, "")

	if suite.T().Failed() {
		suite.T().FailNow()
	}
}

// TestDNSResolveStaticHost verifies that static host entries declared in the
// machine configuration are answered by the host DNS server.
func (suite *CommonSuite) TestDNSResolveStaticHost() {
	if suite.Airgapped {
		suite.T().Skip("skipping test in airgapped mode")
	}

	const (
		staticName = "static-host.test.talos"
		staticIP   = "10.123.45.67"
	)

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	nodeName := k8sNode.Name

	suite.T().Logf("applying static host entry on %s/%s", node, nodeName)

	nodeCtx := client.WithNode(suite.ctx, node)

	cfgDocument := network.NewStaticHostConfigV1Alpha1(staticIP)
	cfgDocument.Hostnames = []string{staticName}

	suite.PatchMachineConfig(nodeCtx, cfgDocument)

	defer suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.StaticHostKind, staticIP)

	podDef, err := suite.NewPod("dns-static-host-test")
	suite.Require().NoError(err)

	podDef = podDef.WithNodeName(nodeName)

	suite.Require().NoError(podDef.Create(suite.ctx, 5*time.Minute))

	defer podDef.Delete(suite.ctx) //nolint:errcheck

	_, stderr, err := podDef.Exec(suite.ctx, "apk add --update bind-tools")
	suite.Require().NoError(err)
	suite.Require().Empty(stderr, "stderr: %s", stderr)

	// Retry — applying the config patch + propagating it to the DNS handler
	// is asynchronous.
	suite.Require().NoError(retry.Constant(60*time.Second, retry.WithUnits(2*time.Second)).Retry(func() error {
		stdout, stderr, err := podDef.Exec(
			suite.ctx,
			"dig +short @"+constants.HostDNSAddress+" "+staticName,
		)
		if err != nil {
			return retry.ExpectedErrorf("dig failed: %v (stderr: %s)", err, stderr)
		}

		if !strings.Contains(stdout, staticIP) {
			return retry.ExpectedErrorf("expected %s in dig output, got %q", staticIP, stdout)
		}

		return nil
	}))
}

// TestBaseOCISpec verifies that the base OCI spec can be modified.
func (suite *CommonSuite) TestBaseOCISpec() {
	if suite.Cluster != nil && suite.Cluster.Provisioner() == base.ProvisionerDocker {
		suite.T().Skip("skipping ulimits test since provisioner is docker")
	}

	if testing.Short() {
		suite.T().Skip("skipping test in short mode.")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	nodeName := k8sNode.Name
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("adjusting base OCI specs on %s/%s", node, nodeName)

	ociRuntimeOverride := criconfig.NewCRIBaseRuntimeSpecConfigV1Alpha1()
	ociRuntimeOverride.OverridesConfig.Object = map[string]any{
		"process": map[string]any{
			"rlimits": []map[string]any{
				{
					"type": "RLIMIT_NOFILE",
					"hard": 1024,
					"soft": 1024,
				},
			},
		},
	}

	suite.PatchMachineConfig(nodeCtx, ociRuntimeOverride)

	ts := suite.LatestServiceEventTimestamp(suite.ctx, "cri", node)

	suite.AssertServiceEventsInOrder(suite.ctx, node, "cri", ts, []string{
		"Stopping",
		"Finished",
		"Starting",
		"Waiting",
		"Preparing",
		"Running",
	})

	ociUlimits1PodDef, err := suite.NewPod("oci-ulimits-test-1")
	suite.Require().NoError(err)

	ociUlimits1PodDef = ociUlimits1PodDef.WithNodeName(nodeName)

	suite.Require().NoError(ociUlimits1PodDef.Create(suite.ctx, 5*time.Minute))

	defer func() { suite.Assert().NoError(ociUlimits1PodDef.Delete(suite.ctx)) }()

	stdout, stderr, err := ociUlimits1PodDef.Exec(
		suite.ctx,
		"ulimit -n",
	)
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)
	suite.Require().Equal("1024\n", stdout)

	// Delete immediately before switching to the CRIBaseRuntimeSpecConfig document.
	suite.Assert().NoError(ociUlimits1PodDef.Delete(suite.ctx))

	suite.RemoveMachineConfigDocuments(nodeCtx, criconfig.CRIBaseRuntimeSpecConfigKind)

	ts = suite.LatestServiceEventTimestamp(suite.ctx, "cri", node)

	suite.AssertServiceEventsInOrder(suite.ctx, node, "cri", ts, []string{
		"Stopping",
		"Finished",
		"Starting",
		"Waiting",
		"Preparing",
		"Running",
	})

	ociUlimits2PodDef, err := suite.NewPod("oci-ulimits-test-2")
	suite.Require().NoError(err)

	ociUlimits2PodDef = ociUlimits2PodDef.WithNodeName(nodeName)

	suite.Require().NoError(ociUlimits2PodDef.Create(suite.ctx, 5*time.Minute))

	defer func() { suite.Assert().NoError(ociUlimits2PodDef.Delete(suite.ctx)) }()

	stdout, stderr, err = ociUlimits2PodDef.Exec(
		suite.ctx,
		"ulimit -n",
	)
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)
	suite.Require().Equal("1048576\n", stdout)
}

func init() {
	allSuites = append(allSuites, &CommonSuite{})
}
