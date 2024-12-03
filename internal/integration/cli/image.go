// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"regexp"
	"strings"
	"testing"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// ImageSuite verifies the image command.
type ImageSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ImageSuite) SuiteName() string {
	return "cli.ImageSuite"
}

// TestDefault verifies default Talos list of images.
func (suite *ImageSuite) TestDefault() {
	suite.RunCLI([]string{"image", "default"},
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("registry.k8s.io/kube-apiserver"))),
	)
}

// TestList verifies listing images in the CRI.
func (suite *ImageSuite) TestList() {
	suite.RunCLI([]string{"image", "ls", "--nodes", suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)},
		base.StdoutShouldMatch(regexp.MustCompile(`IMAGE\s+DIGEST\s+SIZE`)),
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("registry.k8s.io/kube-apiserver"))),
	)

	suite.RunCLI([]string{"image", "ls", "--namespace", "system", "--nodes", suite.RandomDiscoveredNodeInternalIP()},
		base.StdoutShouldMatch(regexp.MustCompile(`IMAGE\s+DIGEST\s+SIZE`)),
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("ghcr.io/siderolabs/kubelet:"))),
	)
}

var imageCacheQuery = []string{"get", "imagecacheconfig", "--output", "jsonpath='{.spec.copyStatus}'"}

// TestPull verifies pulling images to the CRI.
func (suite *ImageSuite) TestPull() {
	const image = "registry.k8s.io/kube-apiserver:v1.27.0"

	node := suite.RandomDiscoveredNodeInternalIP()

	if stdout, _ := suite.RunCLI(imageCacheQuery); strings.Contains(stdout, "ready") {
		suite.T().Logf("skipping as the image cache is present")

		return
	}

	suite.RunCLI([]string{"image", "pull", "--nodes", node, image},
		base.StdoutEmpty(),
	)

	// verify that pulled image appeared, also image aliases should appear
	suite.RunCLI([]string{"image", "ls", "--nodes", node},
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta(image))),
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("sha256:89b8d9dbef2b905b7d028ca8b7f79d35ebd9baa66b0a3ee2ddd4f3e0e2804b45"))),
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("registry.k8s.io/kube-apiserver@sha256:89b8d9dbef2b905b7d028ca8b7f79d35ebd9baa66b0a3ee2ddd4f3e0e2804b45"))),
	)
}

// TestCacheCreate verifies creating a cache tarball.
func (suite *ImageSuite) TestCacheCreate() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	stdOut, _ := suite.RunCLI([]string{"image", "default"})

	imagesList := strings.Split(strings.Trim(stdOut, "\n"), "\n")

	imagesArgs := xslices.Map(imagesList[:2], func(image string) string {
		return "--images=" + image
	})

	cacheFile := suite.T().TempDir() + "/cache.tar"

	args := []string{"image", "cache-create", "--image-cache-path", cacheFile}

	args = append(args, imagesArgs...)

	suite.RunCLI(args, base.StdoutEmpty(), base.StderrNotEmpty())
}

func init() {
	allSuites = append(allSuites, new(ImageSuite))
}
