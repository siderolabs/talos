// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_k8s

package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// VersionSuite verifies Talos version.
type VersionSuite struct {
	base.K8sSuite
}

// SuiteName ...
func (suite *VersionSuite) SuiteName() string {
	return "k8s.VersionSuite"
}

// TestExpectedVersion verifies that node versions matches expected.
func (suite *VersionSuite) TestExpectedVersion() {
	// verify k8s version (api server)
	apiServerVersion, err := suite.DiscoveryClient.ServerVersion()
	suite.Require().NoError(err)

	expectedAPIServerVersion := fmt.Sprintf("v%s", constants.DefaultKubernetesVersion)
	suite.Assert().Equal(expectedAPIServerVersion, apiServerVersion.GitVersion)

	checkKernelVersion := suite.Capabilities().RunsTalosKernel

	// verify each node (kubelet version, Talos version, etc.)
	nodes, err := suite.Clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	suite.Require().NoError(err)

	expectedTalosVersion := fmt.Sprintf("Talos (%s)", suite.Version)
	expectedContainerRuntimeVersion := fmt.Sprintf("containerd://%s", constants.DefaultContainerdVersion)
	expectedKubeletVersion := fmt.Sprintf("v%s", constants.DefaultKubernetesVersion)
	expectedKernelVersion := constants.DefaultKernelVersion

	for _, node := range nodes.Items {
		suite.Assert().Equal(expectedTalosVersion, node.Status.NodeInfo.OSImage)
		suite.Assert().Equal("linux", node.Status.NodeInfo.OperatingSystem)
		suite.Assert().Equal(expectedContainerRuntimeVersion, node.Status.NodeInfo.ContainerRuntimeVersion)
		suite.Assert().Equal(expectedKubeletVersion, node.Status.NodeInfo.KubeletVersion)

		if checkKernelVersion {
			suite.Assert().Equal(expectedKernelVersion, node.Status.NodeInfo.KernelVersion)
		}
	}
}

func init() {
	allSuites = append(allSuites, new(VersionSuite))
}
