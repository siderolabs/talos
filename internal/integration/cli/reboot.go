// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli
// +build integration_cli

package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// RebootSuite tests reboot command.
type RebootSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *RebootSuite) SuiteName() string {
	return "cli.RebootSuite"
}

// TestReboot tests rebooting nodes.
func (suite *RebootSuite) TestReboot() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	controlPlaneNode := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)
	workerNode := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodes := fmt.Sprintf(
		"%s,%s",
		controlPlaneNode,
		workerNode,
	)

	suite.RunCLI([]string{"reboot", "-n", nodes, "--debug"}, base.StdoutMatchFunc(func(stdout string) error {
		if strings.Contains(stdout, "method is not supported") {
			suite.T().Skip("reboot is not supported")
		}

		var err error

		for _, node := range []string{controlPlaneNode, workerNode} {
			if !strings.Contains(stdout, fmt.Sprintf("%v: events check condition met", node)) {
				err = multierror.Append(err, fmt.Errorf("events check condition not met on %v", node))
			}

			if !strings.Contains(stdout, fmt.Sprintf("%v: post check passed", node)) {
				err = multierror.Append(err, fmt.Errorf("post check not passed on %v", node))
			}
		}

		return err
	}))
}

func init() {
	allSuites = append(allSuites, new(RebootSuite))
}
