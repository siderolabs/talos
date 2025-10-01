// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	mc "github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// Sysctl to use for testing config changes.
// This sysctl should:
//
//   - default to a non-Go-null value in the kernel
//
//   - not be defined by the default Talos configuration
//
//   - be generally harmless
const applyConfigTestSysctl = "net.ipv6.conf.all.accept_ra_mtu"

const applyConfigTestSysctlVal = "1"

const applyConfigNoRebootTestSysctl = "fs.file-max"

const applyConfigNoRebootTestSysctlVal = "500000"

const assertRebootedRebootTimeout = 10 * time.Minute

// ApplyConfigSuite ...
type ApplyConfigSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *ApplyConfigSuite) SuiteName() string {
	return "api.ApplyConfigSuite"
}

// SetupTest ...
func (suite *ApplyConfigSuite) SetupTest() {
	// make sure we abort at some point in time, but give enough room for Recovers
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *ApplyConfigSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestApply verifies the apply config API.
func (suite *ApplyConfigSuite) TestApply() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot")
	}

	suite.WaitForBootDone(suite.ctx)

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.T().Logf("applying configuration to node %q", node)
	suite.ClearConnectionRefused(suite.ctx, node)
	nodeCtx := client.WithNode(suite.ctx, node)

	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoErrorf(err, "failed to read existing config from node %q", node)

	cfgDataOut := suite.PatchV1Alpha1Config(provider, func(cfg *v1alpha1.Config) {
		if cfg.MachineConfig.MachineSysctls == nil {
			cfg.MachineConfig.MachineSysctls = make(map[string]string)
		}

		cfg.MachineConfig.MachineSysctls[applyConfigTestSysctl] = applyConfigTestSysctlVal
	})

	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			_, err = suite.Client.ApplyConfiguration(
				nodeCtx, &machineapi.ApplyConfigurationRequest{
					Data: cfgDataOut,
					Mode: machineapi.ApplyConfigurationRequest_REBOOT,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to apply configuration (node %q): %w", node, err)
			}

			return nil
		}, assertRebootedRebootTimeout,
		suite.CleanupFailedPods,
	)

	// Verify configuration change
	var newProvider config.Provider

	suite.Require().NoErrorf(
		retry.Constant(time.Minute, retry.WithUnits(time.Second)).Retry(
			func() error {
				newProvider, err = suite.ReadConfigFromNode(nodeCtx)
				if err != nil {
					return retry.ExpectedError(err)
				}

				return nil
			},
		), "failed to read updated configuration from node %q", node,
	)

	suite.Assert().Equal(
		newProvider.Machine().Sysctls()[applyConfigTestSysctl],
		applyConfigTestSysctlVal,
		"expected sysctl %s to be set to %s, got %s on node %q",
		applyConfigTestSysctl, applyConfigTestSysctlVal, newProvider.Machine().Sysctls()[applyConfigTestSysctl], node,
	)
}

// TestApplyNoOpCRIPatch verifies the apply config with no-op CRI patch.
func (suite *ApplyConfigSuite) TestApplyNoOpCRIPatch() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot")
	}

	suite.WaitForBootDone(suite.ctx)

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.T().Logf("applying configuration to node %q", node)
	suite.ClearConnectionRefused(suite.ctx, node)
	nodeCtx := client.WithNode(suite.ctx, node)

	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoErrorf(err, "failed to read existing config from node %q", node)

	// this CRI patch is a no-op, as NRI is already disabled by default, this verifies that CRI config generation handles it correctly.
	cfgDataOut := suite.PatchV1Alpha1Config(provider, func(cfg *v1alpha1.Config) {
		cfg.MachineConfig.MachineFiles = xslices.Filter(cfg.MachineConfig.MachineFiles, func(file *v1alpha1.MachineFile) bool {
			return file.FilePath != "/etc/cri/conf.d/20-customization.part"
		})

		cfg.MachineConfig.MachineFiles = append(cfg.MachineConfig.MachineFiles,
			&v1alpha1.MachineFile{
				FilePath: "/etc/cri/conf.d/20-customization.part",
				FileOp:   "create",
				FileContent: `[plugins]
          [plugins."io.containerd.nri.v1.nri"]
             disable = true`,
			},
		)
	})

	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			_, err = suite.Client.ApplyConfiguration(
				nodeCtx, &machineapi.ApplyConfigurationRequest{
					Data: cfgDataOut,
					Mode: machineapi.ApplyConfigurationRequest_REBOOT,
				},
			)
			suite.Assert().NoErrorf(err, "failed to apply configuration (node %q)", node)

			return nil
		}, assertRebootedRebootTimeout,
		suite.CleanupFailedPods,
	)

	suite.ClearConnectionRefused(suite.ctx, node)

	// revert the patch
	provider, err = suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoErrorf(err, "failed to read existing config from node %q", node)

	// this CRI patch is a no-op, as NRI is already disabled by default, this verifies that CRI config generation handles it correctly.
	cfgDataOut = suite.PatchV1Alpha1Config(provider, func(cfg *v1alpha1.Config) {
		cfg.MachineConfig.MachineFiles = xslices.Filter(cfg.MachineConfig.MachineFiles, func(file *v1alpha1.MachineFile) bool {
			return file.FilePath != "/etc/cri/conf.d/20-customization.part"
		})
	})

	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			_, err = suite.Client.ApplyConfiguration(
				nodeCtx, &machineapi.ApplyConfigurationRequest{
					Data: cfgDataOut,
					Mode: machineapi.ApplyConfigurationRequest_REBOOT,
				},
			)
			suite.Assert().NoErrorf(err, "failed to apply configuration (node %q)", node)

			return nil
		}, assertRebootedRebootTimeout,
		suite.CleanupFailedPods,
	)
}

// TestApplyWithoutReboot verifies the apply config API without reboot.
func (suite *ApplyConfigSuite) TestApplyWithoutReboot() {
	for _, mode := range []machineapi.ApplyConfigurationRequest_Mode{
		machineapi.ApplyConfigurationRequest_AUTO,
		machineapi.ApplyConfigurationRequest_STAGED,
	} {
		suite.WaitForBootDone(suite.ctx)

		node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
		suite.T().Logf("applying configuration to node %q", node)
		suite.ClearConnectionRefused(suite.ctx, node)
		nodeCtx := client.WithNode(suite.ctx, node)

		provider, err := suite.ReadConfigFromNode(nodeCtx)
		suite.Require().NoError(err, "failed to read existing config from node %q", node)

		cfgDataOut := suite.PatchV1Alpha1Config(provider, func(cfg *v1alpha1.Config) {
			if cfg.MachineConfig.MachineSysctls == nil {
				cfg.MachineConfig.MachineSysctls = make(map[string]string)
			}

			cfg.MachineConfig.MachineSysctls[applyConfigNoRebootTestSysctl] = applyConfigNoRebootTestSysctlVal
		})

		_, err = suite.Client.ApplyConfiguration(
			nodeCtx, &machineapi.ApplyConfigurationRequest{
				Data: cfgDataOut,
				Mode: mode,
			},
		)
		suite.Require().NoError(err, "failed to apply deferred configuration (node %q)", node)

		// Verify configuration change
		var newProvider config.Provider

		newProvider, err = suite.ReadConfigFromNode(nodeCtx)

		suite.Require().NoError(err, "failed to read updated configuration from node %q", node)

		if mode == machineapi.ApplyConfigurationRequest_AUTO {
			suite.Assert().Equal(
				newProvider.Machine().Sysctls()[applyConfigNoRebootTestSysctl],
				applyConfigNoRebootTestSysctlVal,
			)
		} else {
			suite.Assert().NotContains(newProvider.Machine().Sysctls(), applyConfigNoRebootTestSysctl)
		}

		cfgDataOut = suite.PatchV1Alpha1Config(provider, func(cfg *v1alpha1.Config) {
			// revert back
			delete(cfg.MachineConfig.MachineSysctls, applyConfigNoRebootTestSysctl)
		})

		_, err = suite.Client.ApplyConfiguration(
			nodeCtx, &machineapi.ApplyConfigurationRequest{
				Data: cfgDataOut,
				Mode: mode,
			},
		)
		suite.Require().NoError(err, "failed to apply deferred configuration (node %q)", node)
	}
}

// TestApplyConfigRotateEncryptionSecrets verify key rotation by sequential apply config calls.
func (suite *ApplyConfigSuite) TestApplyConfigRotateEncryptionSecrets() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.ClearConnectionRefused(suite.ctx, node)

	nodeCtx := client.WithNode(suite.ctx, node)

	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	ephemeralCfg, _ := provider.Volumes().ByName(constants.EphemeralPartitionLabel)
	encryption := ephemeralCfg.Encryption()

	if encryption == nil {
		suite.T().Skip("skipped in not encrypted mode")
	}

	suite.WaitForBootDone(suite.ctx)

	suite.T().Logf("testing encryption key rotation on node %s", node)

	cfg, ok := encryption.(blockcfg.EncryptionSpec)
	suite.Require().True(ok, "expected blockcfg.EncryptionSpec, got %T", encryption)

	existing := cfg.EncryptionKeys[0]
	slot := existing.Slot() + 1

	keySets := [][]blockcfg.EncryptionKey{
		{
			existing,
			{
				KeyStatic: &blockcfg.EncryptionKeyStatic{
					KeyData: "AlO93jayutOpsDxDS=-",
				},
				KeySlot: slot,
			},
		},
		{
			{
				KeyStatic: &blockcfg.EncryptionKeyStatic{
					KeyData: "AlO93jayutOpsDxDS=-",
				},
				KeySlot: slot,
			},
		},
		{
			existing,
			{
				KeyStatic: &blockcfg.EncryptionKeyStatic{
					KeyData: "AlO93jayutOpsDxDS=-",
				},
				KeySlot: slot,
			},
		},
		{
			existing,
			{
				KeyNodeID: &blockcfg.EncryptionKeyNodeID{},
				KeySlot:   slot,
			},
		},
		{
			{
				KeyNodeID: &blockcfg.EncryptionKeyNodeID{},
				KeySlot:   slot,
			},
		},
	}

	for _, keys := range keySets {
		suite.T().Logf("applying encryption keys %s on node %s", toJSONString(suite.T(), keys), node)

		// prepare a patch to apply, first removing existing keys
		removeKeysPatch := map[string]any{
			"apiVersion": "v1alpha1",
			"kind":       "VolumeConfig",
			"name":       constants.EphemeralPartitionLabel,
			"encryption": map[string]any{
				"keys": map[string]any{
					"$patch": "delete",
				},
			},
		}

		newEphemeralCfg := blockcfg.NewVolumeConfigV1Alpha1()
		newEphemeralCfg.MetaName = constants.EphemeralPartitionLabel
		newEphemeralCfg.EncryptionSpec.EncryptionKeys = keys

		// right now, patching encryption keys doesn't reboot and doesn't rotate the secrets either
		suite.PatchMachineConfig(nodeCtx, removeKeysPatch, newEphemeralCfg)

		suite.AssertRebooted(
			suite.ctx, node,
			func(nodeCtx context.Context) error {
				return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
			}, assertRebootedRebootTimeout,
			suite.CleanupFailedPods,
		)

		suite.ClearConnectionRefused(suite.ctx, node)

		// Verify configuration change
		var newProvider config.Provider

		suite.Require().NoError(
			retry.Constant(time.Minute, retry.WithUnits(time.Second)).Retry(
				func() error {
					newProvider, err = suite.ReadConfigFromNode(nodeCtx)
					if err != nil {
						return retry.ExpectedError(err)
					}

					return nil
				},
			), "failed to read updated configuration from node %q", node,
		)

		newEphemeral, _ := newProvider.Volumes().ByName(constants.EphemeralPartitionLabel)
		e := newEphemeral.Encryption()

		for i, k := range e.Keys() {
			if keys[i].KeyStatic == nil {
				suite.Require().Nil(k.Static())
			} else {
				suite.Require().Equal(keys[i].Static().Key(), k.Static().Key())
			}

			if keys[i].KeyNodeID == nil {
				suite.Require().Nil(k.NodeID())
			} else {
				suite.Require().NotNil(keys[i].NodeID())
			}

			suite.Require().Equal(keys[i].Slot(), k.Slot())
			suite.Require().Equal(keys[i].Slot(), k.Slot())
		}

		suite.WaitForBootDone(suite.ctx)

		// verify that encryption key sync has no failures
		rtestutils.AssertAll(nodeCtx, suite.T(), suite.Client.COSI, func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			suite.Assert().Contains([]block.VolumePhase{block.VolumePhaseReady, block.VolumePhaseMissing}, vs.TypedSpec().Phase)
			suite.Assert().Empty(vs.TypedSpec().EncryptionFailedSyncs)
		})
	}
}

func toJSONString(t *testing.T, v any) string {
	t.Helper()

	out, err := json.Marshal(v)
	require.NoError(t, err)

	return string(out)
}

// TestApplyNoReboot verifies the apply config API fails if NoReboot mode is requested on a field that can not be applied immediately.
func (suite *ApplyConfigSuite) TestApplyNoReboot() {
	suite.WaitForBootDone(suite.ctx)

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.T().Logf("applying configuration to node %q", node)
	suite.ClearConnectionRefused(suite.ctx, node)
	nodeCtx := client.WithNode(suite.ctx, node)

	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoErrorf(err, "failed to read existing config from node %q: %s", node, err)

	cfgDataOut := suite.PatchV1Alpha1Config(provider, func(cfg *v1alpha1.Config) {
		// this won't be possible without a reboot
		cfg.MachineConfig.MachineType = "controlplane"
	})

	_, err = suite.Client.ApplyConfiguration(
		nodeCtx, &machineapi.ApplyConfigurationRequest{
			Data: cfgDataOut,
			Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
		},
	)
	suite.Require().Error(err)

	suite.Require().Equal(codes.InvalidArgument, client.StatusCode(err))
}

// TestApplyDryRun verifies the apply config API with dry run enabled.
func (suite *ApplyConfigSuite) TestApplyDryRun() {
	suite.WaitForBootDone(suite.ctx)

	node := suite.RandomDiscoveredNodeInternalIP()
	suite.T().Logf("applying configuration to node %q", node)
	suite.ClearConnectionRefused(suite.ctx, node)
	nodeCtx := client.WithNode(suite.ctx, node)

	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoErrorf(err, "failed to read existing config from node %q: %s", node, err)

	cfgDataOut := suite.PatchV1Alpha1Config(provider, func(cfg *v1alpha1.Config) {
		// this won't be possible without a reboot
		cfg.MachineConfig.MachineFiles = append(cfg.MachineConfig.MachineFiles,
			&v1alpha1.MachineFile{
				FileContent:     "test",
				FilePermissions: v1alpha1.FileMode(os.ModePerm),
				FilePath:        "/var/lib/test",
				FileOp:          "create",
			},
		)
	})

	reply, err := suite.Client.ApplyConfiguration(
		nodeCtx, &machineapi.ApplyConfigurationRequest{
			Data:   cfgDataOut,
			Mode:   machineapi.ApplyConfigurationRequest_AUTO,
			DryRun: true,
		},
	)

	suite.Require().NoErrorf(err, "failed to apply configuration (node %q): %s", node, err)
	suite.Assert().Contains(reply.Messages[0].ModeDetails, "Dry run summary")
}

// TestApplyDryRunDocuments verifies the apply config API with multi doc and dry run enabled.
func (suite *ApplyConfigSuite) TestApplyDryRunDocuments() {
	suite.WaitForBootDone(suite.ctx)

	node := suite.RandomDiscoveredNodeInternalIP()
	suite.T().Logf("applying configuration to node %q", node)
	suite.ClearConnectionRefused(suite.ctx, node)
	nodeCtx := client.WithNode(suite.ctx, node)

	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoErrorf(err, "failed to read existing config from node %q: %s", node, err)

	kmsg := runtime.NewKmsgLogV1Alpha1()
	kmsg.MetaName = "omni-kmsg"
	kmsg.KmsgLogURL.URL = ensure.Value(url.Parse("tcp://[fdae:41e4:649b:9303::1]:8092"))

	cont, err := container.New(provider.RawV1Alpha1(), kmsg)
	suite.Require().NoErrorf(err, "failed to create container: %s", err)

	cfgDataOut, err := cont.Bytes()
	suite.Require().NoErrorf(err, "failed to marshal container: %s", err)

	reply, err := suite.Client.ApplyConfiguration(
		nodeCtx, &machineapi.ApplyConfigurationRequest{
			Data:   cfgDataOut,
			Mode:   machineapi.ApplyConfigurationRequest_AUTO,
			DryRun: true,
		},
	)

	suite.Require().NoErrorf(err, "failed to apply configuration (node %q): %s", node, err)
	suite.Assert().Contains(reply.Messages[0].ModeDetails, "Dry run summary")
	suite.Assert().Contains(reply.Messages[0].ModeDetails, "omni-kmsg")
	suite.Assert().Contains(reply.Messages[0].ModeDetails, "tcp://[fdae:41e4:649b:9303::1]:8092")
}

// TestApplyTry applies the config in try mode with a short timeout.
func (suite *ApplyConfigSuite) TestApplyTry() {
	suite.WaitForBootDone(suite.ctx)

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.T().Logf("applying configuration to node %q", node)
	suite.ClearConnectionRefused(suite.ctx, node)
	nodeCtx := client.WithNode(suite.ctx, node)

	dummyCfg := network.NewDummyLinkConfigV1Alpha1("dummy-try")

	suite.PatchMachineConfigWithModeSetter(nodeCtx, func(acr *machineapi.ApplyConfigurationRequest) {
		acr.Mode = machineapi.ApplyConfigurationRequest_TRY
		acr.TryModeTimeout = durationpb.New(time.Second * 10)
	}, dummyCfg)

	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoErrorf(err, "failed to read existing config from node %q", node)

	assertDummyInterface := func(provider config.Provider) bool {
		docs := provider.Documents()

		for _, doc := range docs {
			if doc.Kind() == network.DummyLinkKind {
				if namedDocument, ok := doc.(configconfig.NamedDocument); ok && namedDocument.Name() == "dummy-try" {
					return true
				}
			}
		}

		return false
	}

	suite.Assert().Truef(assertDummyInterface(provider), "dummy interface wasn't found")

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, mc.ActiveID, func(r *mc.MachineConfig, asrt *assert.Assertions) {
		asrt.False(assertDummyInterface(r.Provider()))
	})
}

// TestApplyRemovingV1Alpha1 verifies the apply config doesn't accept removal of v1alpha1 config.
func (suite *ApplyConfigSuite) TestApplyRemovingV1Alpha1() {
	suite.WaitForBootDone(suite.ctx)

	node := suite.RandomDiscoveredNodeInternalIP()
	suite.T().Logf("applying configuration to node %q", node)
	suite.ClearConnectionRefused(suite.ctx, node)
	nodeCtx := client.WithNode(suite.ctx, node)

	// create a simple multi-doc config without v1alpha1
	cfgDocument := runtime.NewWatchdogTimerV1Alpha1()
	cfgDocument.WatchdogDevice = "/dev/watchdog0"
	cfgDocument.WatchdogTimeout = 120 * time.Second

	ctr, err := container.New(cfgDocument)
	suite.Require().NoError(err, "failed to create container")

	cfgDataOut, err := ctr.Bytes()
	suite.Require().NoError(err, "failed to marshal container")

	// Talos should deny a request that effectively removes the v1alpha1 config
	_, err = suite.Client.ApplyConfiguration(
		nodeCtx, &machineapi.ApplyConfigurationRequest{
			Data: cfgDataOut,
			Mode: machineapi.ApplyConfigurationRequest_AUTO,
		},
	)
	suite.Require().Error(err)
	suite.Require().Equal(codes.InvalidArgument, client.StatusCode(err))
	suite.Require().ErrorContains(err, "the applied machine configuration doesn't contain v1alpha1 config")
}

func init() {
	allSuites = append(allSuites, new(ApplyConfigSuite))
}
