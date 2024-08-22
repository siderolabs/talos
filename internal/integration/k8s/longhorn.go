// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_k8s

package k8s

import (
	"context"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
)

// LongHornSuite tests deploying Longhorn.
type LongHornSuite struct {
	base.K8sSuite
}

// SuiteName returns the name of the suite.
func (suite *LongHornSuite) SuiteName() string {
	return "k8s.LongHornSuite"
}

// TestDeploy tests deploying Longhorn and running a simple test.
func (suite *LongHornSuite) TestDeploy() {
	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reaching out to the node IP is not reliable")
	}

	if suite.CSITestName != "longhorn" {
		suite.T().Skip("skipping longhorn test as it is not enabled")
	}

	timeout, err := time.ParseDuration(suite.CSITestTimeout)
	if err != nil {
		suite.T().Fatalf("failed to parse timeout: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	suite.T().Cleanup(cancel)

	if err := suite.HelmInstall(
		ctx,
		"longhorn-system",
		"https://charts.longhorn.io",
		LongHornHelmChartVersion,
		"longhorn",
		"longhorn",
		nil,
	); err != nil {
		suite.T().Fatalf("failed to install Longhorn chart: %v", err)
	}

	suite.Require().NoError(suite.RunFIOTest(ctx, "longhorn", "10G"))
}

func init() {
	allSuites = append(allSuites, new(LongHornSuite))
}
