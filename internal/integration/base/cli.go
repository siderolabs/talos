// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli
// +build integration_cli

package base

import (
	"context"
	"fmt"
	"math/rand"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-cmd/pkg/cmd"
	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
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

	discoveredNodes = cliSuite.discoverKubectl()
	if discoveredNodes != nil {
		return discoveredNodes
	}

	// still no nodes, skip the test
	cliSuite.T().Skip("no nodes were discovered")

	return nil
}

// RandomDiscoveredNode returns a random node of the specified type (or any type if no types are specified).
func (cliSuite *CLISuite) RandomDiscoveredNode(types ...machine.Type) string {
	nodeInfo := cliSuite.DiscoverNodes(context.TODO())

	var nodes []string

	if len(types) == 0 {
		nodes = nodeInfo.Nodes()
	} else {
		for _, t := range types {
			nodes = append(nodes, nodeInfo.NodesByType(t)...)
		}
	}

	cliSuite.Require().NotEmpty(nodes)

	return nodes[rand.Intn(len(nodes))]
}

func (cliSuite *CLISuite) discoverKubectl() cluster.Info {
	// pull down kubeconfig into temporary directory
	tempDir := cliSuite.T().TempDir()

	// rely on `nodes:` being set in talosconfig
	cliSuite.RunCLI([]string{"kubeconfig", tempDir}, StdoutEmpty())

	masterNodes, err := cmd.Run(cliSuite.KubectlPath, "--kubeconfig", filepath.Join(tempDir, "kubeconfig"), "get", "nodes",
		"-o", "jsonpath={.items[*].status.addresses[?(@.type==\"InternalIP\")].address}", fmt.Sprintf("--selector=%s", constants.LabelNodeRoleMaster))
	cliSuite.Require().NoError(err)

	workerNodes, err := cmd.Run(cliSuite.KubectlPath, "--kubeconfig", filepath.Join(tempDir, "kubeconfig"), "get", "nodes",
		"-o", "jsonpath={.items[*].status.addresses[?(@.type==\"InternalIP\")].address}", fmt.Sprintf("--selector=!%s", constants.LabelNodeRoleMaster))
	cliSuite.Require().NoError(err)

	return &infoWrapper{
		masterNodes: strings.Fields(strings.TrimSpace(masterNodes)),
		workerNodes: strings.Fields(strings.TrimSpace(workerNodes)),
	}
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
func (cliSuite *CLISuite) RunCLI(args []string, options ...RunOption) (stdout string) {
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
			return retry.ExpectedError(fmt.Errorf("stdout doesn't match: %q", stdout))
		}

		return nil
	}))
}
