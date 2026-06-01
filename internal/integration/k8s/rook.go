// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_k8s

package k8s

import (
	"context"
	_ "embed"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
)

//go:embed testdata/rook-ceph-cluster-values.yaml
var rookCephClusterValues []byte

// RookSuite tests deploying Rook.
type RookSuite struct {
	base.K8sSuite
}

// SuiteName returns the name of the suite.
func (suite *RookSuite) SuiteName() string {
	return "k8s.RookSuite"
}

// TestDeploy tests deploying Rook and running a simple test.
func (suite *RookSuite) TestDeploy() {
	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reaching out to the node IP is not reliable")
	}

	if suite.CSITestName != "rook-ceph" {
		suite.T().Skip("skipping rook-ceph test as it is not enabled")
	}

	timeout, err := time.ParseDuration(suite.CSITestTimeout)
	if err != nil {
		suite.T().Fatalf("failed to parse timeout: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	suite.T().Cleanup(cancel)

	if err := suite.HelmInstall(
		ctx,
		"rook-ceph",
		"https://charts.rook.io/release",
		RookCephHelmChartVersion,
		"rook-ceph",
		"rook-ceph",
		nil,
	); err != nil {
		suite.T().Fatalf("failed to install Rook chart: %v", err)
	}

	if err := suite.HelmInstall(
		ctx,
		"rook-ceph",
		"https://charts.rook.io/release",
		RookCephHelmChartVersion,
		"rook-ceph-cluster",
		"rook-ceph-cluster",
		rookCephClusterValues,
	); err != nil {
		suite.T().Fatalf("failed to install Rook chart: %v", err)
	}

	if err := suite.WaitForResource(ctx, "rook-ceph", "ceph.rook.io", "CephCluster", "v1", "rook-ceph", "{.status.phase}", "Ready"); err != nil {
		suite.T().Fatalf("failed to wait for CephCluster to be Ready: %v", err)
	}

	if err := suite.WaitForResource(ctx, "rook-ceph", "ceph.rook.io", "CephCluster", "v1", "rook-ceph", "{.status.state}", "Created"); err != nil {
		suite.T().Fatalf("failed to wait for CephCluster to be Created: %v", err)
	}

	if err := suite.WaitForResource(ctx, "rook-ceph", "ceph.rook.io", "CephCluster", "v1", "rook-ceph", "{.status.ceph.health}", "HEALTH_OK"); err != nil {
		suite.T().Fatalf("failed to wait for CephCluster to be HEALTH_OK: %v", err)
	}

	suite.Require().NoError(suite.RunFIOTest(ctx, "ceph-block", "10G"))
}

func init() {
	allSuites = append(allSuites, new(RookSuite))
}
