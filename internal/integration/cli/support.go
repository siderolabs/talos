// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"archive/zip"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
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

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	suite.RunCLI([]string{"support", "--nodes", node, "-w", "5", "-O", output},
		base.StderrNotEmpty(),
	)

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
		"service-logs/apid.log",
		"service-logs/apid.state",
		"service-logs/machined.log",
		"service-logs/machined.state",
		"service-logs/kubelet.log",
		"service-logs/kubelet.state",
		"resources/kernelparamstatuses.runtime.talos.dev.yaml",
		"kubernetes-logs/kube-system/kube-apiserver.log",
		"controller-runtime.log",
		"mounts",
		"processes",
		"io",
		"summary",
	} {
		n := fmt.Sprintf("%s/%s", node, name)
		suite.Require().Contains(files, n, "File %s doesn't exist in the support bundle", n)
	}

	for _, name := range []string{
		"kubernetesResources/nodes.yaml",
		"kubernetesResources/systemPods.yaml",
	} {
		suite.Require().Contains(files, name, "File %s doesn't exist in the support bundle", name)
	}
}

func init() {
	allSuites = append(allSuites, new(SupportSuite))
}
