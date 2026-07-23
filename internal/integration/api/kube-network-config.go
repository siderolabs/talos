// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	k8sres "github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// podCIDRFamilies reports which IP families are present among the cluster's pod CIDRs.
func podCIDRFamilies(netCfg configconfig.K8sNetworkConfig) (hasIPv4, hasIPv6 bool) {
	for _, podCIDR := range netCfg.PodCIDRs() {
		if podCIDR.Addr().Is6() {
			hasIPv6 = true
		} else {
			hasIPv4 = true
		}
	}

	return hasIPv4, hasIPv6
}

// KubeNetworkConfigSuite verifies the KubeNetworkConfig node CIDR mask size settings
// are wired end-to-end into the kube-controller-manager arguments.
type KubeNetworkConfigSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *KubeNetworkConfigSuite) SuiteName() string {
	return "api.KubeNetworkConfigSuite"
}

// SetupTest ...
func (suite *KubeNetworkConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)
}

// TearDownTest ...
func (suite *KubeNetworkConfigSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestNodeCIDRMaskSize applies a KubeNetworkConfig node CIDR mask size for each address family
// present in the cluster's pod CIDRs and asserts the value flows through to the rendered
// kube-controller-manager arguments, then reverts to the defaults.
//
// The test handles both legacy (.machine.cluster.network) and multidoc (KubeNetworkConfig) clusters
// by reading the effective config, building a complete KubeNetworkConfig document, atomically
// clearing the legacy field if present, and restoring the original config in cleanup.
func (suite *KubeNetworkConfigSuite) TestNodeCIDRMaskSize() {
	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping if cluster is not qemu")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing on node %q", node)

	// Read the current config to restore it at the end
	originalProvider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoErrorf(err, "failed to read existing config from node %q", node)

	originalCfgBytes, err := originalProvider.Bytes()
	suite.Require().NoErrorf(err, "failed to marshal original config for node %q", node)

	// Restore original config when the test finishes
	defer func() {
		if err := suite.restoreConfig(nodeCtx, originalCfgBytes); err != nil {
			suite.T().Logf("failed to restore original config: %v", err)
		}
	}()

	netCfg := originalProvider.K8sNetworkConfig()
	suite.Require().NotNil(netCfg, "cluster has no K8sNetworkConfig")

	hasIPv4, hasIPv6 := podCIDRFamilies(netCfg)
	suite.Require().True(hasIPv4 || hasIPv6, "cluster has no pod CIDRs")

	const (
		customIPv4MaskSize = 25
		customIPv6MaskSize = 72
	)

	// Apply custom (non-default) node CIDR mask sizes for the families present, and assert they
	// reach the rendered kube-controller-manager arguments.
	suite.applyNodeCIDRMaskSizes(nodeCtx, originalProvider, netCfg, customIPv4MaskSize, customIPv6MaskSize)
	suite.assertControllerManagerArgs(nodeCtx, netCfg, customIPv4MaskSize, customIPv6MaskSize)

	// Revert to the defaults and assert they take effect again.
	suite.applyNodeCIDRMaskSizes(nodeCtx, originalProvider, netCfg, constants.DefaultNodeCIDRMaskSizeIPv4, constants.DefaultNodeCIDRMaskSizeIPv6)
	suite.assertControllerManagerArgs(nodeCtx, netCfg, constants.DefaultNodeCIDRMaskSizeIPv4, constants.DefaultNodeCIDRMaskSizeIPv6)
}

// applyNodeCIDRMaskSizes builds a complete KubeNetworkConfig document with the given mask sizes,
// clears the legacy v1alpha1 .cluster.network field if present, and applies both in a single request.
func (suite *KubeNetworkConfigSuite) applyNodeCIDRMaskSizes(
	nodeCtx context.Context,
	provider config.Provider,
	netCfg configconfig.K8sNetworkConfig,
	ipv4MaskSize int,
	ipv6MaskSize int,
) {
	kubeNetCfg := k8s.NewKubeNetworkConfigV1Alpha1()
	kubeNetCfg.NetworkDNSDomain = netCfg.DNSDomain()
	kubeNetCfg.NetworkPodSubnets = xslices.Map(netCfg.PodCIDRs(), func(p netip.Prefix) meta.Prefix {
		return meta.Prefix{Prefix: p}
	})
	kubeNetCfg.NetworkServiceSubnets = xslices.Map(netCfg.ServiceCIDRs(), func(p netip.Prefix) meta.Prefix {
		return meta.Prefix{Prefix: p}
	})
	kubeNetCfg.NetworkNodeCIDRMaskSizeIPv4 = ipv4MaskSize
	kubeNetCfg.NetworkNodeCIDRMaskSizeIPv6 = ipv6MaskSize

	// Clear the legacy v1alpha1 field if present
	patched, err := provider.PatchV1Alpha1(func(cfg *v1alpha1.Config) error {
		if cfg.ClusterConfig != nil {
			cfg.ClusterConfig.ClusterNetwork = nil //nolint:staticcheck // intentionally clearing legacy field
		}

		return nil
	})
	suite.Require().NoError(err, "failed to patch v1alpha1 config")

	// Drop any pre-existing KubeNetworkConfig document (multi-doc clusters ship one by
	// default) so it doesn't collide with the one we just built.
	existingDocs := slices.DeleteFunc(patched.Documents(), func(d configconfig.Document) bool {
		return d.Kind() == k8s.KubeNetworkConfig
	})

	// Build the final config with both the legacy-field clear and the new document
	cont, err := container.New(slices.Concat(existingDocs, []configconfig.Document{kubeNetCfg})...)
	suite.Require().NoError(err, "failed to build config container")

	cfgDataOut, err := cont.Bytes()
	suite.Require().NoError(err, "failed to marshal config")

	_, err = suite.Client.ApplyConfiguration(
		nodeCtx, &machineapi.ApplyConfigurationRequest{
			Data: cfgDataOut,
			Mode: machineapi.ApplyConfigurationRequest_AUTO,
		},
	)
	suite.Require().NoError(err, "failed to apply configuration")
}

// assertControllerManagerArgs waits for the rendered kube-controller-manager arguments to contain
// the expected per-family node CIDR mask size flags.
func (suite *KubeNetworkConfigSuite) assertControllerManagerArgs(nodeCtx context.Context, netCfg configconfig.K8sNetworkConfig, ipv4MaskSize int, ipv6MaskSize int) {
	hasIPv4, hasIPv6 := podCIDRFamilies(netCfg)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, k8sres.FinalControllerManagerConfigID,
		func(cfg *k8sres.ControllerManagerConfig, asrt *assert.Assertions) {
			if hasIPv4 {
				asrt.Contains(cfg.TypedSpec().Args, fmt.Sprintf("--node-cidr-mask-size-ipv4=%d", ipv4MaskSize))
			}

			if hasIPv6 {
				asrt.Contains(cfg.TypedSpec().Args, fmt.Sprintf("--node-cidr-mask-size-ipv6=%d", ipv6MaskSize))
			}
		},
	)
}

// restoreConfig applies the original configuration back to the node.
func (suite *KubeNetworkConfigSuite) restoreConfig(nodeCtx context.Context, originalCfgBytes []byte) error {
	_, err := suite.Client.ApplyConfiguration(
		nodeCtx, &machineapi.ApplyConfigurationRequest{
			Data: originalCfgBytes,
			Mode: machineapi.ApplyConfigurationRequest_AUTO,
		},
	)

	return err
}

func init() {
	allSuites = append(allSuites, new(KubeNetworkConfigSuite))
}
