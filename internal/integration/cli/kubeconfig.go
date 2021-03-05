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

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
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

	defer os.RemoveAll(tempDir) //nolint:errcheck

	suite.RunCLI([]string{"kubeconfig", "--merge=false", "--nodes", suite.RandomDiscoveredNode(machine.TypeControlPlane), tempDir},
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

	defer os.RemoveAll(tempDir) //nolint:errcheck

	savedCwd, err := os.Getwd()
	suite.Require().NoError(err)

	defer os.Chdir(savedCwd) //nolint:errcheck

	suite.Require().NoError(os.Chdir(tempDir))

	suite.RunCLI([]string{"kubeconfig", "--merge=false", "--nodes", suite.RandomDiscoveredNode(machine.TypeControlPlane)},
		base.StdoutEmpty())

	suite.Require().FileExists(filepath.Join(tempDir, "kubeconfig"))
}

// TestCustomName generates kubeconfig with custom name.
func (suite *KubeconfigSuite) TestCustomName() {
	tempDir, err := ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	defer os.RemoveAll(tempDir) //nolint:errcheck

	suite.RunCLI([]string{"kubeconfig", "--merge=false", "--nodes", suite.RandomDiscoveredNode(machine.TypeControlPlane), filepath.Join(tempDir, "k8sconfig")},
		base.StdoutEmpty())

	suite.Require().FileExists(filepath.Join(tempDir, "k8sconfig"))
}

// TestMultiNodeFail verifies that command fails with multiple nodes.
func (suite *KubeconfigSuite) TestMultiNodeFail() {
	suite.RunCLI([]string{"kubeconfig", "--merge=false", "--nodes", "127.0.0.1", "--nodes", "127.0.0.1", "."},
		base.ShouldFail(),
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`is not supported with multiple nodes`)))
}

// TestMergeRename tests merge config into existing kubeconfig with default rename conflict resolution.
func (suite *KubeconfigSuite) TestMergeRename() {
	tempDir, err := ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	defer os.RemoveAll(tempDir) //nolint:errcheck

	path := filepath.Join(tempDir, "config")

	suite.RunCLI([]string{"kubeconfig", "--nodes", suite.RandomDiscoveredNode(machine.TypeControlPlane), path},
		base.StdoutEmpty())
	suite.RunCLI([]string{"kubeconfig", "--nodes", suite.RandomDiscoveredNode(machine.TypeControlPlane), path})

	config, err := clientcmd.LoadFromFile(path)
	suite.Require().NoError(err)

	suite.Require().Equal(len(config.Contexts), 2)
}

// TestMergeOverwrite test merge config into existing kubeconfig with overwrite conflict resolution.
func (suite *KubeconfigSuite) TestMergeOverwrite() {
	tempDir, err := ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	defer os.RemoveAll(tempDir) //nolint:errcheck

	path := filepath.Join(tempDir, "config")

	suite.RunCLI([]string{"kubeconfig", "--nodes", suite.RandomDiscoveredNode(machine.TypeControlPlane), path},
		base.StdoutEmpty())
	suite.RunCLI([]string{"kubeconfig", "--force", "--nodes", suite.RandomDiscoveredNode(machine.TypeControlPlane), path},
		base.StdoutEmpty())

	config, err := clientcmd.LoadFromFile(path)
	suite.Require().NoError(err)

	suite.Require().Equal(len(config.Contexts), 1)
}

func init() {
	allSuites = append(allSuites, new(KubeconfigSuite))
}
