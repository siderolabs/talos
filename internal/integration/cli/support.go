// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli
// +build integration_cli

package cli

import (
	"archive/zip"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// SupportSuite verifies support command.
type SupportSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *SupportSuite) SuiteName() string {
	return "cli.SupportSuite"
}

// TestSupport does successful support run.
func (suite *SupportSuite) TestSupport() {
	tempDir := suite.T().TempDir()

	output := filepath.Join(tempDir, "support.zip")

	node := suite.RandomDiscoveredNode(machine.TypeControlPlane)

	suite.RunCLI([]string{"support", "--nodes", node, "-w", "5", "-O", output})

	archive, err := zip.OpenReader(output)
	suite.Require().NoError(err)

	defer archive.Close() //nolint:errcheck

	files := map[string]struct{}{}

	for _, f := range archive.File {
		files[f.Name] = struct{}{}

		if strings.HasSuffix(f.Name, "dmesg.log") {
			suite.Require().Greater(f.UncompressedSize64, uint64(0), "dmesg log is empty")
		}
	}

	for _, name := range []string{
		"dmesg.log",
		"controller-runtime.log",
		"apid.log",
		"apid.state",
		"machined.log",
		"machined.state",
		"kubelet.log",
		"kubelet.state",
		"talosResources/kernelparamstatuses.runtime.talos.dev.yaml",
		"kube-system/kube-apiserver.log",
		"mounts",
		"processes",
		"io",
		"summary",
	} {
		n := fmt.Sprintf("%s/%s", node, name)
		suite.Require().Contains(files, n, "File %s doesn't exist in the support bundle", n)
	}

	for _, name := range []string{
		"cluster/kubernetesResources/nodes.yaml",
		"cluster/kubernetesResources/systemPods.yaml",
	} {
		suite.Require().Contains(files, name, "File %s doesn't exist in the support bundle", name)
	}
}

func init() {
	allSuites = append(allSuites, new(SupportSuite))
}
