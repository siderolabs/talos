// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/integration/base"
)

// KubeconfigSuite verifies dmesg command.
type KubeconfigSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *KubeconfigSuite) SuiteName() string {
	return "cli.KubeconfigSuite"
}

// TestDirectory generates kubeconfig in specified directory.
func (suite *KubeconfigSuite) TestDirectory() {
	tempDir, err := ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	defer os.RemoveAll(tempDir) //nolint: errcheck

	suite.RunCLI([]string{"kubeconfig", "--nodes", suite.RandomDiscoveredNode(runtime.MachineTypeControlPlane), tempDir},
		base.StdoutEmpty())

	path := filepath.Join(tempDir, "kubeconfig")
	suite.Require().FileExists(path)

	_, err = clientcmd.LoadFromFile(path)
	suite.Require().NoError(err)
}

// TestCwd generates kubeconfig in cwd.
func (suite *KubeconfigSuite) TestCwd() {
	tempDir, err := ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	defer os.RemoveAll(tempDir) //nolint: errcheck

	savedCwd, err := os.Getwd()
	suite.Require().NoError(err)

	defer os.Chdir(savedCwd) //nolint: errcheck

	suite.Require().NoError(os.Chdir(tempDir))

	suite.RunCLI([]string{"kubeconfig", "--nodes", suite.RandomDiscoveredNode(runtime.MachineTypeControlPlane)},
		base.StdoutEmpty())

	suite.Require().FileExists(filepath.Join(tempDir, "kubeconfig"))
}

// TestMultiNodeFail verifies that command fails with multiple nodes.
func (suite *KubeconfigSuite) TestMultiNodeFail() {
	suite.RunCLI([]string{"kubeconfig", "--nodes", "127.0.0.1", "--nodes", "127.0.0.1", "."},
		base.ShouldFail(),
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`is not supported with multiple nodes`)))
}

// TestMerge test merge config into existing kubeconfig
func (suite *KubeconfigSuite) TestMerge() {
	tempDir, err := ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	defer os.RemoveAll(tempDir) //nolint: errcheck

	path := filepath.Join(tempDir, "config")

	suite.RunCLI([]string{"kubeconfig", "--nodes", suite.RandomDiscoveredNode(runtime.MachineTypeControlPlane), path, "-m"},
		base.StdoutEmpty())
	suite.RunCLI([]string{"kubeconfig", "--nodes", suite.RandomDiscoveredNode(runtime.MachineTypeControlPlane), path, "-m"})

	config, err := clientcmd.LoadFromFile(path)
	suite.Require().NoError(err)

	suite.Require().Equal(len(config.Contexts), 2)
}

func init() {
	allSuites = append(allSuites, new(KubeconfigSuite))
}
