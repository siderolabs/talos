// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package base

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/cluster/check"
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

// RandomNode returns a random node of the specified type (or any type if no types are specified).
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
	defer clusterAccess.Close() //nolint: errcheck

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

	defer reader.Close() //nolint: errcheck

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}

	bootID := string(body)

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
			return retry.ExpectedError(fmt.Errorf("bootID didn't change for node %q: before %s + %s, after %s", node, bootIDBefore, bootIDAfter))
		}

		return nil
	}))

	if apiSuite.Cluster != nil {
		// without cluster state we can't do deep checks, but basic reboot test still works
		// NB: using `ctx` here to have client talking to init node by default
		apiSuite.AssertClusterHealthy(ctx)
	}
}

// TearDownSuite closes Talos API client.
func (apiSuite *APISuite) TearDownSuite() {
	if apiSuite.Client != nil {
		apiSuite.Assert().NoError(apiSuite.Client.Close())
	}
}
