// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"context"
	"regexp"
	"strings"

	"github.com/siderolabs/talos/internal/integration/base"
)

// testContentLibrary is the content library hack/test/patches/content-library.yaml declares, which the
// content library CI job applies. Clusters created without that patch have no library at all.
const testContentLibrary = "test-content-library"

// ContentLibrarySuite verifies the talosctl hypervisor content-library commands.
type ContentLibrarySuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ContentLibrarySuite) SuiteName() string {
	return "cli.ContentLibrarySuite"
}

// TestListUnknownLibrary asserts that listing a library which is not configured fails, rather than
// reporting an empty library.
func (suite *ContentLibrarySuite) TestListUnknownLibrary() {
	node := suite.RandomDiscoveredNodeInternalIP()

	suite.RunCLI(
		[]string{"hypervisor", "content-library", "list", "--nodes", node, "not-a-library"},
		base.ShouldFail(),
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
	)
}

// TestDeleteUnknownLibrary asserts the same for delete.
func (suite *ContentLibrarySuite) TestDeleteUnknownLibrary() {
	node := suite.RandomDiscoveredNodeInternalIP()

	suite.RunCLI(
		[]string{"hypervisor", "content-library", "delete", "--nodes", node, "not-a-library", "image.raw"},
		base.ShouldFail(),
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
	)
}

// TestUploadRequiresSingleNode asserts that an upload refuses to fan out: the same image written to
// every node of a cluster is never what was meant.
func (suite *ContentLibrarySuite) TestUploadRequiresSingleNode() {
	nodes := suite.DiscoverNodeInternalIPs(context.TODO())

	if len(nodes) < 2 {
		suite.T().Skip("skipping test, the cluster has a single node")
	}

	suite.RunCLI(
		[]string{"hypervisor", "content-library", "upload", "--nodes", nodes[0], "--nodes", nodes[1], "not-a-library", "/etc/hosts"},
		base.ShouldFail(),
		base.StderrShouldMatch(regexp.MustCompile(`requires exactly one node`)),
		base.StdoutEmpty(),
	)
}

// TestRoundTrip covers the CLI against a real library: what is uploaded is listed back and can be
// deleted again.
func (suite *ContentLibrarySuite) TestRoundTrip() {
	node := suite.RandomDiscoveredNodeInternalIP()

	if !suite.libraryConfigured(node) {
		suite.T().Skipf("skipping test, node %s has no content library %q configured", node, testContentLibrary)
	}

	const name = "cli-test.raw"

	nameRegexp := regexp.MustCompile(regexp.QuoteMeta(name))

	// From stdin, which is the only way the upload takes a name of its own.
	suite.RunCLI(
		[]string{"hypervisor", "content-library", "upload", "--nodes", node, "--name", name, "--overwrite", testContentLibrary, "-"},
		base.WithStdin(strings.NewReader("talos")),
		base.StdoutShouldMatch(nameRegexp),
	)

	suite.RunCLI(
		[]string{"hypervisor", "content-library", "list", "--nodes", node, testContentLibrary},
		base.StdoutShouldMatch(nameRegexp),
	)

	suite.RunCLI(
		[]string{"hypervisor", "content-library", "delete", "--nodes", node, testContentLibrary, name},
		base.StdoutEmpty(),
	)

	suite.RunCLI(
		[]string{"hypervisor", "content-library", "list", "--nodes", node, testContentLibrary},
		base.StdoutShouldNotMatch(nameRegexp),
	)
}

// libraryConfigured reports whether the node has the default content library.
func (suite *ContentLibrarySuite) libraryConfigured(node string) bool {
	return suite.MakeCMDFn([]string{"get", "--nodes", node, "contentlibrarystatus", testContentLibrary})().Run() == nil
}

func init() {
	allSuites = append(allSuites, new(ContentLibrarySuite))
}
