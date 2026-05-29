// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	_ "embed"
	"regexp"
	"strings"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/version"
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
	suite.RunCLI(
		[]string{"image", "k8s-bundle"},
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("registry.k8s.io/kube-apiserver"))),
	)
}

var versionRe = regexp.MustCompile(`^[v]?(\d+\.\d+\.\d+(?:-(?:alpha|beta|rc)\.\d+)?)`)

func normalizeTag(tag string) string {
	m := versionRe.FindStringSubmatch(tag)
	if len(m) == 0 {
		return tag
	}

	return m[1]
}

var (
	//go:embed testdata/images/talos-bundle-1.11.2.txt
	bundle1_11_2 string

	//go:embed testdata/images/talos-bundle-1.14.0-alpha.1.txt
	bundle1_14_0Alpha1 string
)

// TestTalosBundle verifies talos-bundle Talos list of images.
func (suite *ImageSuite) TestTalosBundle() {
	for rawTag, out := range map[string]string{
		"v1.11.2":         bundle1_11_2,
		"v1.14.0-alpha.1": bundle1_14_0Alpha1,
	} {
		suite.RunCLI(
			[]string{"image", "talos-bundle", rawTag},
			base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta(out))),
		)
	}

	tag, err := semver.ParseTolerant(normalizeTag(version.Tag))
	assert.NoError(suite.T(), err)

	suite.T().Log(normalizeTag(version.Tag))
	suite.T().Log(version.Tag)

	if strings.TrimLeft(version.Tag, "v") == normalizeTag(version.Tag) {
		suite.T().Skip("skipping the test for the exact version tag")
	}

	suite.RunCLI(
		[]string{"image", "talos-bundle", "v" + tag.String()},
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("ghcr.io/siderolabs/talos:v"+tag.String()))),
	)

	suite.RunCLI(
		[]string{"image", "talos-bundle", tag.String()},
		base.StdoutEmpty(),
		base.ShouldFail(),
	)

	tag.Patch = 0
	assert.NoError(suite.T(), tag.IncrementMinor())
	suite.RunCLI(
		[]string{"image", "talos-bundle", "v" + tag.FinalizeVersion()},
		base.StdoutEmpty(),
		base.ShouldFail(),
	)
}

// TestList verifies listing images in the CRI.
func (suite *ImageSuite) TestList() {
	suite.RunCLI(
		[]string{"image", "ls", "--nodes", suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)},
		base.StdoutShouldMatch(regexp.MustCompile(`IMAGE\s+DIGEST\s+SIZE`)),
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("registry.k8s.io/kube-apiserver"))),
	)

	suite.RunCLI(
		[]string{"image", "ls", "--namespace", "system", "--nodes", suite.RandomDiscoveredNodeInternalIP()},
		base.StdoutShouldMatch(regexp.MustCompile(`IMAGE\s+DIGEST\s+SIZE`)),
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("ghcr.io/siderolabs/kubelet:"))),
	)
}

// TestPull verifies pulling images to the CRI.
func (suite *ImageSuite) TestPull() {
	const image = "registry.k8s.io/kube-apiserver:v1.27.0" // sync this to e2e.sh `build_image_cache`

	node := suite.RandomDiscoveredNodeInternalIP()

	if stdout, _ := suite.RunCLI([]string{"get", "imagecacheconfig", "--nodes", node, "--output", "jsonpath='{.spec.status}'"}); strings.Contains(stdout, "ready") {
		suite.T().Logf("skipping as the image cache is present")

		return
	}

	suite.RunCLI(
		[]string{"image", "pull", "--nodes", node, image},
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(
			"("+ // either pinned image (verification on)
				regexp.QuoteMeta("pulled image registry.k8s.io/kube-apiserver@sha256:89b8d9dbef2b905b7d028ca8b7f79d35ebd9baa66b0a3ee2ddd4f3e0e2804b45")+
				"|"+ // or original ref (without verification)
				regexp.QuoteMeta("pulled image "+image)+
				")",
		)),
	)

	// verify that pulled image appeared, also image aliases should appear
	suite.RunCLI(
		[]string{"image", "ls", "--nodes", node},
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta(image))),
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("sha256:89b8d9dbef2b905b7d028ca8b7f79d35ebd9baa66b0a3ee2ddd4f3e0e2804b45"))),
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta("registry.k8s.io/kube-apiserver@sha256:89b8d9dbef2b905b7d028ca8b7f79d35ebd9baa66b0a3ee2ddd4f3e0e2804b45"))),
		base.StdoutShouldMatch(regexp.MustCompile(regexp.QuoteMeta(image))),
	)

	// remove the image
	suite.RunCLI(
		[]string{"image", "remove", "--nodes", node, image},
		base.StdoutEmpty(),
	)
}

// TestCacheCreateOCI verifies creating a cache tarball.
func (suite *ImageSuite) TestCacheCreateOCI() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	stdOut, _ := suite.RunCLI([]string{"image", "k8s-bundle"})

	imagesList := strings.Split(strings.Trim(stdOut, "\n"), "\n")

	imagesArgs := xslices.Map(imagesList[:2], func(image string) string {
		return "--images=" + image
	})

	cacheDir := suite.T().TempDir()

	args := []string{"image", "cache-create", "--image-cache-path", cacheDir, "--force", "--layout", "oci"} //nolint:prealloc // this is a test

	args = append(args, imagesArgs...)

	suite.RunCLI(args, base.StdoutEmpty(), base.StderrNotEmpty())

	assert.FileExistsf(suite.T(), cacheDir+"/index.json", "index.json should exist in the image cache directory")
}

// TestCacheCreateFlat verifies creating a cache directory.
func (suite *ImageSuite) TestCacheCreateFlat() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	stdOut, _ := suite.RunCLI([]string{"image", "k8s-bundle"})

	imagesList := strings.Split(strings.Trim(stdOut, "\n"), "\n")

	imagesArgs := xslices.Map(imagesList[:2], func(image string) string {
		return "--images=" + image
	})

	cacheDir := suite.T().TempDir()

	args := []string{"image", "cache-create", "--image-cache-path", cacheDir, "--force", "--layout", "flat"} //nolint:prealloc // this is a test

	args = append(args, imagesArgs...)

	suite.RunCLI(args, base.StdoutEmpty(), base.StderrNotEmpty())

	assert.DirExistsf(suite.T(), cacheDir+"/blob", "blob directory should exist in the image cache directory")
	assert.DirExistsf(suite.T(), cacheDir+"/manifests", "manifests directory should exist in the image cache directory")
}

func init() {
	allSuites = append(allSuites, new(ImageSuite))
}
