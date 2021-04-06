// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package base

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"
	"google.golang.org/grpc/backoff"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/cluster/check"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/provision"
	"github.com/talos-systems/talos/pkg/provision/access"
)

// APISuite is a base suite for API tests.
type APISuite struct {
	suite.Suite
	TalosSuite

	Client *client.Client
}

// SetupSuite initializes Talos API client.
func (apiSuite *APISuite) SetupSuite() {
	cfg, err := config.Open(apiSuite.TalosConfig)
	apiSuite.Require().NoError(err)

	opts := []client.OptionFunc{
		client.WithConfig(cfg),
	}

	if apiSuite.Endpoint != "" {
		opts = append(opts, client.WithEndpoints(apiSuite.Endpoint))
	}

	apiSuite.Client, err = client.New(context.TODO(), opts...)
	apiSuite.Require().NoError(err)

	// clear any connection refused errors left after the previous tests
	nodes := apiSuite.DiscoverNodes().Nodes()

	if len(nodes) > 0 {
		// grpc might trigger backoff on reconnect attempts, so make sure we clear them
		apiSuite.ClearConnectionRefused(context.Background(), nodes...)
	}
}

// DiscoverNodes provides list of Talos nodes in the cluster.
//
// As there's no way to provide this functionality via Talos API, it works the following way:
// 1. If there's a provided cluster info, it's used.
// 2. If integration test was compiled with k8s support, k8s is used.
func (apiSuite *APISuite) DiscoverNodes() cluster.Info {
	discoveredNodes := apiSuite.TalosSuite.DiscoverNodes()
	if discoveredNodes != nil {
		return discoveredNodes
	}

	var err error

	apiSuite.discoveredNodes, err = discoverNodesK8s(apiSuite.Client, &apiSuite.TalosSuite)
	apiSuite.Require().NoError(err, "k8s discovery failed")

	if apiSuite.discoveredNodes == nil {
		// still no nodes, skip the test
		apiSuite.T().Skip("no nodes were discovered")
	}

	return apiSuite.discoveredNodes
}

// RandomDiscoveredNode returns a random node of the specified type (or any type if no types are specified).
func (apiSuite *APISuite) RandomDiscoveredNode(types ...machine.Type) string {
	nodeInfo := apiSuite.DiscoverNodes()

	var nodes []string

	if len(types) == 0 {
		nodes = nodeInfo.Nodes()
	} else {
		for _, t := range types {
			nodes = append(nodes, nodeInfo.NodesByType(t)...)
		}
	}

	apiSuite.Require().NotEmpty(nodes)

	return nodes[rand.Intn(len(nodes))]
}

// Capabilities describes current cluster allowed actions.
type Capabilities struct {
	RunsTalosKernel bool
	SupportsReboot  bool
	SupportsRecover bool
}

// Capabilities returns a set of capabilities to skip tests for different environments.
func (apiSuite *APISuite) Capabilities() Capabilities {
	v, err := apiSuite.Client.Version(context.Background())
	apiSuite.Require().NoError(err)

	caps := Capabilities{}

	if v.Messages[0].Platform != nil {
		switch v.Messages[0].Platform.Mode {
		case runtime.ModeContainer.String():
		default:
			caps.RunsTalosKernel = true
			caps.SupportsReboot = true
			caps.SupportsRecover = true
		}
	}

	return caps
}

// AssertClusterHealthy verifies that cluster is healthy using provisioning checks.
func (apiSuite *APISuite) AssertClusterHealthy(ctx context.Context) {
	if apiSuite.Cluster == nil {
		// can't assert if cluster state was provided
		apiSuite.T().Skip("cluster health can't be verified when cluster state is not provided")
	}

	clusterAccess := access.NewAdapter(apiSuite.Cluster, provision.WithTalosClient(apiSuite.Client))
	defer clusterAccess.Close() //nolint:errcheck

	apiSuite.Require().NoError(check.Wait(ctx, clusterAccess, append(check.DefaultClusterChecks(), check.ExtraClusterChecks()...), check.StderrReporter()))
}

// ReadBootID reads node boot_id.
//
// Context provided might have specific node attached for API call.
func (apiSuite *APISuite) ReadBootID(ctx context.Context) (string, error) {
	// set up a short timeout around boot_id read calls to work around
	// cases when rebooted node doesn't answer for a long time on requests
	reqCtx, reqCtxCancel := context.WithTimeout(ctx, 10*time.Second)
	defer reqCtxCancel()

	reader, errCh, err := apiSuite.Client.Read(reqCtx, "/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}

	defer reader.Close() //nolint:errcheck

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}

	bootID := strings.TrimSpace(string(body))

	_, err = io.Copy(ioutil.Discard, reader)
	if err != nil {
		return "", err
	}

	for err = range errCh {
		if err != nil {
			return "", err
		}
	}

	return bootID, reader.Close()
}

// AssertRebooted verifies that node got rebooted as result of running some API call.
//
// Verification happens via reading boot_id of the node.
func (apiSuite *APISuite) AssertRebooted(ctx context.Context, node string, rebootFunc func(nodeCtx context.Context) error, timeout time.Duration) {
	// timeout for single node Reset
	ctx, ctxCancel := context.WithTimeout(ctx, timeout)
	defer ctxCancel()

	nodeCtx := client.WithNodes(ctx, node)

	// read boot_id before Reset
	bootIDBefore, err := apiSuite.ReadBootID(nodeCtx)

	apiSuite.Require().NoError(err)

	apiSuite.Assert().NoError(rebootFunc(nodeCtx))

	var bootIDAfter string

	apiSuite.Require().NoError(retry.Constant(timeout).Retry(func() error {
		requestCtx, requestCtxCancel := context.WithTimeout(nodeCtx, 5*time.Second)
		defer requestCtxCancel()

		bootIDAfter, err = apiSuite.ReadBootID(requestCtx)

		if err != nil {
			// API might be unresponsive during reboot
			return retry.ExpectedError(err)
		}

		if bootIDAfter == bootIDBefore {
			// bootID should be different after reboot
			return retry.ExpectedError(fmt.Errorf("bootID didn't change for node %q: before %s, after %s", node, bootIDBefore, bootIDAfter))
		}

		return nil
	}))

	if apiSuite.Cluster != nil {
		// without cluster state we can't do deep checks, but basic reboot test still works
		// NB: using `ctx` here to have client talking to init node by default
		apiSuite.AssertClusterHealthy(ctx)
	}
}

// WaitForBootDone waits for boot phase done event.
func (apiSuite *APISuite) WaitForBootDone(ctx context.Context) {
	nodes := apiSuite.DiscoverNodes().Nodes()

	nodesNotDoneBooting := make(map[string]struct{})

	for _, node := range nodes {
		nodesNotDoneBooting[node] = struct{}{}
	}

	ctx, cancel := context.WithTimeout(client.WithNodes(ctx, nodes...), 3*time.Minute)
	defer cancel()

	apiSuite.Require().NoError(apiSuite.Client.EventsWatch(ctx, func(ch <-chan client.Event) {
		defer cancel()

		for event := range ch {
			if msg, ok := event.Payload.(*machineapi.SequenceEvent); ok {
				if msg.GetAction() == machineapi.SequenceEvent_STOP && msg.GetSequence() == runtime.SequenceBoot.String() {
					delete(nodesNotDoneBooting, event.Node)

					if len(nodesNotDoneBooting) == 0 {
						return
					}
				}
			}
		}
	}, client.WithTailEvents(-1)))

	apiSuite.Require().Empty(nodesNotDoneBooting)
}

// ClearConnectionRefused clears cached connection refused errors which might be left after node reboot.
func (apiSuite *APISuite) ClearConnectionRefused(ctx context.Context, nodes ...string) {
	ctx, cancel := context.WithTimeout(ctx, backoff.DefaultConfig.MaxDelay)
	defer cancel()

	numMasterNodes := len(apiSuite.DiscoverNodes().NodesByType(machine.TypeControlPlane)) + len(apiSuite.DiscoverNodes().NodesByType(machine.TypeInit))
	if numMasterNodes == 0 {
		numMasterNodes = 3
	}

	apiSuite.Require().NoError(retry.Constant(backoff.DefaultConfig.MaxDelay, retry.WithUnits(time.Second)).Retry(func() error {
		for i := 0; i < numMasterNodes; i++ {
			_, err := apiSuite.Client.Version(client.WithNodes(ctx, nodes...))
			if err == nil {
				continue
			}

			if strings.Contains(err.Error(), "connection refused") {
				return retry.ExpectedError(err)
			}

			return retry.UnexpectedError(err)
		}

		return nil
	}))
}

// HashKubeletCert returns hash of the kubelet certificate file.
//
// This function can be used to verify that the node ephemeral partition got wiped.
func (apiSuite *APISuite) HashKubeletCert(ctx context.Context, node string) (string, error) {
	reqCtx, reqCtxCancel := context.WithTimeout(ctx, 10*time.Second)
	defer reqCtxCancel()

	reqCtx = client.WithNodes(reqCtx, node)

	reader, errCh, err := apiSuite.Client.Read(reqCtx, "/var/lib/kubelet/pki/kubelet-client-current.pem")
	if err != nil {
		return "", err
	}

	defer reader.Close() //nolint:errcheck

	hash := sha256.New()

	_, err = io.Copy(hash, reader)
	if err != nil {
		return "", err
	}

	for err = range errCh {
		if err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), reader.Close()
}

// TearDownSuite closes Talos API client.
func (apiSuite *APISuite) TearDownSuite() {
	if apiSuite.Client != nil {
		apiSuite.Assert().NoError(apiSuite.Client.Close())
	}
}
