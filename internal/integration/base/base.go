// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

// Package base provides shared definition of base suites for tests
package base

import (
	"context"

	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/access"
)

const (
	// ProvisionerQEMU is the name of the QEMU provisioner.
	ProvisionerQEMU = "qemu"
)

// TalosSuite defines most common settings for integration test suites.
type TalosSuite struct {
	// Endpoint to use to connect, if not set config is used
	Endpoint string
	// K8sEndpoint is API server endpoint, if set overrides kubeconfig
	K8sEndpoint string
	// Cluster describes provisioned cluster, used for discovery purposes
	Cluster provision.Cluster
	// TalosConfig is a path to talosconfig
	TalosConfig string
	// Version is the (expected) version of Talos tests are running against
	Version string
	// GoVersion is the (expected) version of Go compiler.
	GoVersion string
	// TalosctlPath is a path to talosctl binary
	TalosctlPath string
	// KubectlPath is a path to kubectl binary
	KubectlPath string
	// HelmPath is a path to helm binary
	HelmPath string
	// KubeStrPath is a path to kubestr binary
	KubeStrPath string
	// ExtensionsQEMU runs tests with qemu and extensions enabled
	ExtensionsQEMU bool
	// ExtensionsNvidia runs tests with nvidia extensions enabled
	ExtensionsNvidia bool
	// TrustedBoot tells if the cluster is secure booted and disks are encrypted
	TrustedBoot bool
	// TalosImage is the image name for 'talos' container.
	TalosImage string
	// CSITestName is the name of the CSI test to run
	CSITestName string
	// CSITestTimeout is the timeout for the CSI test
	CSITestTimeout string

	discoveredNodes cluster.Info
}

// DiscoverNodes provides basic functionality to discover cluster nodes via test settings.
//
// This method is overridden in specific suites to allow for specific discovery.
func (talosSuite *TalosSuite) DiscoverNodes(_ context.Context) cluster.Info {
	if talosSuite.discoveredNodes == nil {
		if talosSuite.Cluster != nil {
			talosSuite.discoveredNodes = access.NewAdapter(talosSuite.Cluster).Info
		}
	}

	return talosSuite.discoveredNodes
}

// ConfiguredSuite expects config to be set before running.
type ConfiguredSuite interface {
	SetConfig(config TalosSuite)
}

// SetConfig implements ConfiguredSuite.
func (talosSuite *TalosSuite) SetConfig(config TalosSuite) {
	*talosSuite = config
}

// NamedSuite interface provides names for test suites.
type NamedSuite interface {
	SuiteName() string
}
