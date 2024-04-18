// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package base

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/siderolabs/go-cmd/pkg/cmd"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// CLISuite is a base suite for CLI tests.
type CLISuite struct {
	suite.Suite
	TalosSuite
}

// DiscoverNodes provides list of Talos nodes in the cluster.
//
// As there's no way to provide this functionality via Talos CLI, it relies on cluster info.
func (cliSuite *CLISuite) DiscoverNodes(ctx context.Context) cluster.Info {
	discoveredNodes := cliSuite.TalosSuite.DiscoverNodes(ctx)
	if discoveredNodes != nil {
		return discoveredNodes
	}

	return cliSuite.discoverKubectl()
}

// DiscoverNodeInternalIPs provides list of Talos node internal IPs in the cluster.
func (cliSuite *CLISuite) DiscoverNodeInternalIPs(ctx context.Context) []string {
	nodes := cliSuite.DiscoverNodes(ctx)

	return mapNodeInfosToInternalIPs(nodes.Nodes())
}

// DiscoverNodeInternalIPsByType provides list of Talos node internal IPs in the cluster for given machine type.
func (cliSuite *CLISuite) DiscoverNodeInternalIPsByType(ctx context.Context, machineType machine.Type) []string {
	nodesByType := cliSuite.DiscoverNodes(ctx).NodesByType(machineType)

	return mapNodeInfosToInternalIPs(nodesByType)
}

// RandomDiscoveredNodeInternalIP returns the internal IP a random node of the specified type (or any type if no types are specified).
func (cliSuite *CLISuite) RandomDiscoveredNodeInternalIP(types ...machine.Type) string {
	nodeInfo := cliSuite.DiscoverNodes(context.TODO())

	var nodes []cluster.NodeInfo

	if len(types) == 0 {
		nodes = nodeInfo.Nodes()
	} else {
		for _, t := range types {
			nodes = append(nodes, nodeInfo.NodesByType(t)...)
		}
	}

	cliSuite.Require().NotEmpty(nodes)

	return nodes[rand.IntN(len(nodes))].InternalIP.String()
}

func (cliSuite *CLISuite) discoverKubectl() cluster.Info {
	// pull down kubeconfig into temporary directory
	tempDir := cliSuite.T().TempDir()

	// rely on `nodes:` being set in talosconfig
	cliSuite.RunCLI([]string{"kubeconfig", tempDir}, StdoutEmpty())

	masterNodes, err := cmd.Run(cliSuite.KubectlPath, "--kubeconfig", filepath.Join(tempDir, "kubeconfig"), "get", "nodes",
		"-o", "jsonpath={.items[*].status.addresses[?(@.type==\"InternalIP\")].address}", fmt.Sprintf("--selector=%s", constants.LabelNodeRoleControlPlane))
	cliSuite.Require().NoError(err)

	workerNodes, err := cmd.Run(cliSuite.KubectlPath, "--kubeconfig", filepath.Join(tempDir, "kubeconfig"), "get", "nodes",
		"-o", "jsonpath={.items[*].status.addresses[?(@.type==\"InternalIP\")].address}", fmt.Sprintf("--selector=!%s", constants.LabelNodeRoleControlPlane))
	cliSuite.Require().NoError(err)

	nodeInfo, err := newNodeInfo(
		strings.Fields(strings.TrimSpace(masterNodes)),
		strings.Fields(strings.TrimSpace(workerNodes)),
	)
	cliSuite.Require().NoError(err)

	return nodeInfo
}

// buildCLICmd builds exec.Cmd from TalosSuite and args.
// TalosSuite flags are added at the beginning so they can be overridden by args.
func (cliSuite *CLISuite) buildCLICmd(args []string) *exec.Cmd {
	if cliSuite.Endpoint != "" {
		args = append([]string{"--endpoints", cliSuite.Endpoint}, args...)
	}

	args = append([]string{"--talosconfig", cliSuite.TalosConfig}, args...)

	return exec.Command(cliSuite.TalosctlPath, args...)
}

// RunCLI runs talosctl binary with the options provided.
func (cliSuite *CLISuite) RunCLI(args []string, options ...RunOption) (stdout, stderr string) {
	return run(&cliSuite.Suite, func() *exec.Cmd { return cliSuite.buildCLICmd(args) }, options...)
}

// RunAndWaitForMatch retries command until output matches.
func (cliSuite *CLISuite) RunAndWaitForMatch(args []string, regex *regexp.Regexp, duration time.Duration, options ...retry.Option) {
	cliSuite.Assert().NoError(retry.Constant(duration, options...).Retry(func() error {
		stdout, _, err := runAndWait(&cliSuite.Suite, cliSuite.buildCLICmd(args))
		if err != nil {
			return err
		}

		if !regex.MatchString(stdout.String()) {
			return retry.ExpectedErrorf("stdout doesn't match: %q", stdout)
		}

		return nil
	}))
}
