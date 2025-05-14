// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	networkconfig "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// EthernetSuite ...
type EthernetSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *EthernetSuite) SuiteName() string {
	return "api.EthernetSuite"
}

// SetupTest ...
func (suite *EthernetSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 1*time.Minute)

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping ethernet test since provisioner is not qemu")
	}
}

// TearDownTest ...
func (suite *EthernetSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

func getFeatureStatus(t *testing.T, features network.EthernetFeatureStatusList, name string) bool {
	t.Helper()

	for _, f := range features {
		if f.Name == name {
			switch f.Status {
			case "on":
				return true
			case "off":
				return false
			default:
				require.Fail(t, "unexpected feature status: %s", f.Status)
			}
		}
	}

	require.Fail(t, "feature %s not found", name)

	panic("unreachable")
}

// TestEthernetConfig verifies changing Ethernet settings.
func (suite *EthernetSuite) TestEthernetConfig() {
	// pick up a random node to test the Ethernet on, and use it throughout the test
	node := suite.RandomDiscoveredNodeInternalIP()

	suite.T().Logf("testing Ethernet on node %s", node)

	// build a Talos API context which is tied to the node
	nodeCtx := client.WithNode(suite.ctx, node)

	// pick a Ethernet links
	ethStatuses, err := safe.StateListAll[*network.EthernetStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	var (
		linkName       string
		prevRingConfig *network.EthernetRingsStatus
		prevFeatures   network.EthernetFeatureStatusList
	)

	for ethStatus := range ethStatuses.All() {
		if ethStatus.TypedSpec().Rings != nil && ethStatus.TypedSpec().Rings.RXMax != nil {
			linkName = ethStatus.Metadata().ID()
			prevRingConfig = ethStatus.TypedSpec().Rings
			prevFeatures = ethStatus.TypedSpec().Features

			marshaled, err := resource.MarshalYAML(ethStatus)
			suite.Require().NoError(err)

			out, err := yaml.Marshal(marshaled)
			suite.Require().NoError(err)

			suite.T().Logf("found link %s with: %s", linkName, string(out))

			break
		}
	}

	suite.Require().NotEmpty(linkName, "no link provides RX rings")
	suite.Require().NotEmpty(prevFeatures, "no link provides features")

	suite.Run("Rings", func() {
		if os.Getenv("CI") != "" {
			suite.T().Skip("skipping ethtool test in CI, as QEMU version doesn't support updating RX rings for virtio")
		}

		// first, adjust RX rings to be 50% of what it was before
		newRX := pointer.SafeDeref(prevRingConfig.RXMax) / 2

		suite.T().Logf("testing RX rings on link %s: %d -> %d", linkName, pointer.SafeDeref(prevRingConfig.RX), newRX)

		cfgDocument := networkconfig.NewEthernetConfigV1Alpha1(linkName)
		cfgDocument.RingsConfig = &networkconfig.EthernetRingsConfig{
			RX: pointer.To(newRX),
		}
		suite.PatchMachineConfig(nodeCtx, cfgDocument)

		// now EthernetStatus should reflect the new RX rings
		rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, linkName,
			func(ethStatus *network.EthernetStatus, asrt *assert.Assertions) {
				asrt.Equal(newRX, pointer.SafeDeref(ethStatus.TypedSpec().Rings.RX))
			},
		)

		// now, let's revert the RX rings to what it was before
		cfgDocument.RingsConfig.RX = prevRingConfig.RX

		suite.PatchMachineConfig(nodeCtx, cfgDocument)

		// now EthernetStatus should reflect the new RX rings
		rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, linkName,
			func(ethStatus *network.EthernetStatus, asrt *assert.Assertions) {
				asrt.Equal(pointer.SafeDeref(prevRingConfig.RX), pointer.SafeDeref(ethStatus.TypedSpec().Rings.RX))
			},
		)

		// remove the config document
		suite.RemoveMachineConfigDocuments(nodeCtx, cfgDocument.MetaKind)
	})

	suite.Run("Features", func() {
		const featureName = "tx-tcp-segmentation"

		// get the initial state
		initialState := getFeatureStatus(suite.T(), prevFeatures, featureName)

		suite.T().Logf("testing feature %s on link %s: %v -> %v", featureName, linkName, initialState, !initialState)

		cfgDocument := networkconfig.NewEthernetConfigV1Alpha1(linkName)
		cfgDocument.FeaturesConfig = map[string]bool{
			featureName: !initialState,
		}
		suite.PatchMachineConfig(nodeCtx, cfgDocument)

		// now EthernetStatus should reflect the new feature status
		rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, linkName,
			func(ethStatus *network.EthernetStatus, asrt *assert.Assertions) {
				asrt.Equal(!initialState, getFeatureStatus(suite.T(), ethStatus.TypedSpec().Features, featureName))
			},
		)

		// now, let's revert the RX rings to what it was before
		cfgDocument.FeaturesConfig[featureName] = initialState

		suite.PatchMachineConfig(nodeCtx, cfgDocument)

		// now EthernetStatus should reflect the old feature status
		rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, linkName,
			func(ethStatus *network.EthernetStatus, asrt *assert.Assertions) {
				asrt.Equal(initialState, getFeatureStatus(suite.T(), ethStatus.TypedSpec().Features, featureName))
			},
		)

		// remove the config document
		suite.RemoveMachineConfigDocuments(nodeCtx, cfgDocument.MetaKind)
	})

	suite.Run("Channels", func() {
		suite.T().Skip("channels are not supported by the current QEMU version")
	})
}

// TestBridgeMAC verifies bridge MAC address.
func (suite *EthernetSuite) TestBridgeMAC() {
	// pick up a random node to test the Ethernet on, and use it throughout the test
	node := suite.RandomDiscoveredNodeInternalIP()

	suite.T().Logf("testing bridge MAC on node %s", node)

	// build a Talos API context which is tied to the node
	nodeCtx := client.WithNode(suite.ctx, node)

	randomSuffix := fmt.Sprintf("%04x", rand.Int32())

	patch1 := v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: v1alpha1.NetworkDeviceList{
					{
						DeviceInterface: "dummy" + randomSuffix,
						DeviceDummy:     pointer.To(true),
					},
					{
						DeviceInterface: "bridge" + randomSuffix,
						DeviceBridge:    &v1alpha1.Bridge{},
					},
				},
			},
		},
	}

	suite.PatchMachineConfig(nodeCtx, patch1)

	var dummyMAC string

	// the links should be created
	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI,
		[]string{"dummy" + randomSuffix, "bridge" + randomSuffix},
		func(link *network.LinkStatus, _ *assert.Assertions) {
			if link.TypedSpec().Kind == "dummy" {
				dummyMAC = link.TypedSpec().HardwareAddr.String()
			}
		})

	suite.Assert().NotEmpty(dummyMAC, "dummy MAC address is empty")

	// now, let's put dummy interface into the bridge
	patch2 := v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: v1alpha1.NetworkDeviceList{
					{
						DeviceInterface: "bridge" + randomSuffix,
						DeviceBridge: &v1alpha1.Bridge{
							BridgedInterfaces: []string{"dummy" + randomSuffix},
						},
					},
				},
			},
		},
	}

	suite.PatchMachineConfig(nodeCtx, patch2)

	// now bridge should have the same MAC address as dummy
	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI,
		[]string{"dummy" + randomSuffix, "bridge" + randomSuffix},
		func(link *network.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal(dummyMAC, link.TypedSpec().HardwareAddr.String(), "dummy MAC address is not equal to bridge MAC address")
		})

	// revert the changes removing the dummy interface from the bridge
	patch3 := map[string]any{
		"machine": map[string]any{
			"network": map[string]any{
				"interfaces": []map[string]any{
					{
						"interface": "bridge" + randomSuffix,
						"$patch":    "delete",
					},
					{
						"interface": "dummy" + randomSuffix,
						"$patch":    "delete",
					},
				},
			},
		},
	}

	suite.PatchMachineConfig(nodeCtx, patch3)

	rtestutils.AssertNoResource[*network.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, "dummy"+randomSuffix)
	rtestutils.AssertNoResource[*network.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, "bridge"+randomSuffix)
}

func init() {
	allSuites = append(allSuites, new(EthernetSuite))
}
