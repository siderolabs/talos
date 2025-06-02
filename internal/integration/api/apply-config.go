// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
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

	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker)
	suite.Require().NotEmpty(nodes)

	suite.WaitForBootDone(suite.ctx)

	slices.Sort(nodes)

	node := nodes[0]
	nodeCtx := client.WithNode(suite.ctx, node)

	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Assert().NoErrorf(err, "failed to read existing config from node %q", node)

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
			suite.Assert().NoErrorf(err, "failed to apply configuration (node %q)", node)

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

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	suite.WaitForBootDone(suite.ctx)

	nodeCtx := client.WithNode(suite.ctx, node)

	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Assert().NoErrorf(err, "failed to read existing config from node %q", node)

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

	// revert the patch
	provider, err = suite.ReadConfigFromNode(nodeCtx)
	suite.Assert().NoErrorf(err, "failed to read existing config from node %q", node)

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

		node := suite.RandomDiscoveredNodeInternalIP()
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

	suite.Assert().NoError(err)

	machineConfig := provider.RawV1Alpha1()
	suite.Assert().NotNil(machineConfig)

	encryption := machineConfig.MachineConfig.MachineSystemDiskEncryption

	if encryption == nil {
		suite.T().Skip("skipped in not encrypted mode")
	}

	cfg := encryption.EphemeralPartition

	if cfg == nil {
		suite.T().Skip("skipped in not encrypted mode")
	}

	provider.Machine().SystemDiskEncryption().Get(constants.EphemeralPartitionLabel)

	suite.WaitForBootDone(suite.ctx)

	suite.T().Logf("testing encryption key rotation on node %s", node)

	existing := cfg.EncryptionKeys[0]
	slot := existing.Slot() + 1

	keySets := [][]*v1alpha1.EncryptionKey{
		{
			existing,
			{
				KeyStatic: &v1alpha1.EncryptionKeyStatic{
					KeyData: "AlO93jayutOpsDxDS=-",
				},
				KeySlot: slot,
			},
		},
		{
			{
				KeyStatic: &v1alpha1.EncryptionKeyStatic{
					KeyData: "AlO93jayutOpsDxDS=-",
				},
				KeySlot: slot,
			},
		},
		{
			existing,
			{
				KeyStatic: &v1alpha1.EncryptionKeyStatic{
					KeyData: "AlO93jayutOpsDxDS=-",
				},
				KeySlot: slot,
			},
		},
		{
			existing,
			{
				KeyNodeID: &v1alpha1.EncryptionKeyNodeID{},
				KeySlot:   slot,
			},
		},
		{
			{
				KeyNodeID: &v1alpha1.EncryptionKeyNodeID{},
				KeySlot:   slot,
			},
		},
	}

	for _, keys := range keySets {
		data := suite.PatchV1Alpha1Config(provider, func(cfg *v1alpha1.Config) {
			suite.T().Logf("changing encryption keys to %s",
				toJSONString(suite.T(), keys),
			)

			cfg.MachineConfig.MachineSystemDiskEncryption.EphemeralPartition.EncryptionKeys = keys
		})

		suite.AssertRebooted(
			suite.ctx, node, func(nodeCtx context.Context) error {
				_, err = suite.Client.ApplyConfiguration(
					nodeCtx, &machineapi.ApplyConfigurationRequest{
						Data: data,
						Mode: machineapi.ApplyConfigurationRequest_REBOOT,
					},
				)
				if err != nil {
					// It is expected that the connection will EOF here, so just log the error
					suite.T().Logf("failed to apply configuration (node %q): %s", node, err)
				}

				return nil
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

		e := newProvider.Machine().SystemDiskEncryption().Get(constants.EphemeralPartitionLabel)

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
	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker)
	suite.Require().NotEmpty(nodes)

	suite.WaitForBootDone(suite.ctx)

	slices.Sort(nodes)

	node := nodes[0]
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
	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker)
	suite.Require().NotEmpty(nodes)

	suite.WaitForBootDone(suite.ctx)

	slices.Sort(nodes)

	node := nodes[0]
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
	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker)
	suite.Require().NotEmpty(nodes)

	suite.WaitForBootDone(suite.ctx)

	slices.Sort(nodes)

	node := nodes[0]
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
	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker)
	suite.Require().NotEmpty(nodes)

	suite.WaitForBootDone(suite.ctx)

	slices.Sort(nodes)

	node := nodes[0]
	nodeCtx := client.WithNode(suite.ctx, node)

	getMachineConfig := func(ctx context.Context) (*mc.MachineConfig, error) {
		cfg, err := safe.StateGetByID[*mc.MachineConfig](ctx, suite.Client.COSI, mc.ActiveID)
		if err != nil {
			return nil, err
		}

		return cfg, nil
	}

	provider, err := getMachineConfig(nodeCtx)
	suite.Require().NoErrorf(err, "failed to read existing config from node %q: %s", node, err)

	cfgDataOut := suite.PatchV1Alpha1Config(provider.Provider(), func(cfg *v1alpha1.Config) {
		if cfg.MachineConfig.MachineNetwork == nil {
			cfg.MachineConfig.MachineNetwork = &v1alpha1.NetworkConfig{}
		}

		cfg.MachineConfig.MachineNetwork.NetworkInterfaces = append(cfg.MachineConfig.MachineNetwork.NetworkInterfaces,
			&v1alpha1.Device{
				DeviceInterface: "dummy0",
				DeviceDummy:     pointer.To(true),
			},
		)
	})

	_, err = suite.Client.ApplyConfiguration(
		nodeCtx, &machineapi.ApplyConfigurationRequest{
			Data:           cfgDataOut,
			Mode:           machineapi.ApplyConfigurationRequest_TRY,
			TryModeTimeout: durationpb.New(time.Second * 1),
		},
	)
	suite.Assert().NoErrorf(err, "failed to apply configuration (node %q)", node)

	provider, err = getMachineConfig(nodeCtx)
	suite.Require().NoErrorf(err, "failed to read existing config from node %q", node)

	suite.Assert().NotNil(provider.Config().Machine().Network())
	suite.Assert().NotNil(provider.Config().Machine().Network().Devices())

	lookupDummyInterface := func() bool {
		for _, device := range provider.Config().Machine().Network().Devices() {
			if device.Dummy() && device.Interface() == "dummy0" {
				return true
			}
		}

		return false
	}

	suite.Assert().Truef(lookupDummyInterface(), "dummy interface wasn't found")

	for range 100 {
		provider, err = getMachineConfig(nodeCtx)
		suite.Assert().NoErrorf(err, "failed to read existing config from node %q: %s", node, err)

		if provider.Config().Machine().Network() == nil {
			return
		}

		if !lookupDummyInterface() {
			return
		}

		time.Sleep(time.Millisecond * 100)
	}

	suite.Fail("dummy interface wasn't removed after config try timeout")
}

// TestApplyRemovingV1Alpha1 verifies the apply config doesn't accept removal of v1alpha1 config.
func (suite *ApplyConfigSuite) TestApplyRemovingV1Alpha1() {
	suite.WaitForBootDone(suite.ctx)

	node := suite.RandomDiscoveredNodeInternalIP()
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
