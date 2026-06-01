// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_k8s

package k8s

import (
	"bytes"
	"context"
	_ "embed"
	"path/filepath"
	"text/template"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

//go:embed testdata/openebs-values.yaml
var openEBSValues []byte

//go:embed testdata/openebs-diskpool.yaml
var openEBSDiskPoolTemplate string

// OpenEBSSuite tests deploying OpenEBS.
type OpenEBSSuite struct {
	base.K8sSuite
}

// SuiteName returns the name of the suite.
func (suite *OpenEBSSuite) SuiteName() string {
	return "k8s.OpenEBSSuite"
}

// TestDeploy tests deploying OpenEBS and running a simple test.
func (suite *OpenEBSSuite) TestDeploy() {
	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reaching out to the node IP is not reliable")
	}

	if suite.CSITestName != "openebs" {
		suite.T().Skip("skipping openebs test as it is not enabled")
	}

	timeout, err := time.ParseDuration(suite.CSITestTimeout)
	if err != nil {
		suite.T().Fatalf("failed to parse timeout: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	suite.T().Cleanup(cancel)

	if err := suite.HelmInstall(
		ctx,
		"openebs",
		"https://openebs.github.io/openebs",
		OpenEBSChartVersion,
		"openebs",
		"openebs",
		openEBSValues,
	); err != nil {
		suite.T().Fatalf("failed to install OpenEBS chart: %v", err)
	}

	nodes := suite.DiscoverNodeInternalIPsByType(ctx, machine.TypeWorker)

	suite.Require().Equal(3, len(nodes), "expected 3 worker nodes")

	disks := xslices.Map(nodes, func(node string) string {
		return suite.UserDisks(ctx, node)[0]
	})

	suite.Require().Equal(3, len(disks), "expected 3 disks")

	for i, disk := range disks {
		node := nodes[i]

		k8sNode, err := suite.GetK8sNodeByInternalIP(ctx, node)
		suite.Require().NoError(err)

		diskResource, err := safe.ReaderGetByID[*block.Disk](client.WithNode(ctx, node), suite.Client.COSI, filepath.Base(disk))
		suite.Require().NoError(err)

		suite.Require().Greater(len(diskResource.TypedSpec().Symlinks), 1, "disk symlinks should not be empty")

		diskSymlink := diskResource.TypedSpec().Symlinks[1]

		tmpl, err := template.New(node).Parse(openEBSDiskPoolTemplate)
		suite.Require().NoError(err)

		var result bytes.Buffer

		suite.Require().NoError(tmpl.Execute(&result, struct {
			Node string
			Disk string
		}{
			Node: k8sNode.Name,
			Disk: diskSymlink,
		}))

		diskPoolUnstructured := suite.ParseManifests(result.Bytes())

		suite.ApplyManifests(ctx, diskPoolUnstructured)
	}

	suite.Require().NoError(suite.RunFIOTest(ctx, "openebs-single-replica", "10G"))
}

func init() {
	allSuites = append(allSuites, new(OpenEBSSuite))
}
