// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/internal/integration/base"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Sysctl to use for testing config changes.
// This sysctl should:
//
//   - default to a non-Go-null value in the kernel
//
//   - not be defined by the default Talos configuration
//
//   - be generally harmless
//
const applyConfigTestSysctl = "net.ipv6.conf.all.accept_ra_mtu"

const applyConfigTestSysctlVal = "1"

const applyConfigNoRebootTestSysctl = "fs.file-max"

const applyConfigNoRebootTestSysctlVal = "500000"

const assertRebootedRebootTimeout = 10 * time.Minute

// ApplyConfigSuite ...
type ApplyConfigSuite struct {
	base.K8sSuite

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *ApplyConfigSuite) SuiteName() string {
	return "api.ApplyConfigSuite"
}

// SetupTest ...
func (suite *ApplyConfigSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

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
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot")
	}

	nodes := suite.DiscoverNodes().NodesByType(machine.TypeJoin)
	suite.Require().NotEmpty(nodes)

	suite.WaitForBootDone(suite.ctx)

	sort.Strings(nodes)

	node := nodes[0]

	nodeCtx := client.WithNodes(suite.ctx, node)

	provider, err := suite.readConfigFromNode(nodeCtx)
	suite.Assert().Nilf(err, "failed to read existing config from node %q: %w", node, err)

	cfg, ok := provider.(*v1alpha1.Config)
	suite.Require().True(ok)

	if cfg.MachineConfig.MachineSysctls == nil {
		cfg.MachineConfig.MachineSysctls = make(map[string]string)
	}

	cfg.MachineConfig.MachineSysctls[applyConfigTestSysctl] = applyConfigTestSysctlVal

	cfgDataOut, err := cfg.Bytes()
	suite.Assert().Nilf(err, "failed to marshal updated machine config data (node %q): %w", node, err)

	suite.AssertRebooted(suite.ctx, node, func(nodeCtx context.Context) error {
		_, err = suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
			Data: cfgDataOut,
		})
		if err != nil {
			// It is expected that the connection will EOF here, so just log the error
			suite.Assert().Nilf("failed to apply configuration (node %q): %w", node, err)
		}

		return nil
	}, assertRebootedRebootTimeout)

	// Verify configuration change
	var newProvider config.Provider

	suite.Require().Nilf(retry.Constant(time.Minute, retry.WithUnits(time.Second)).Retry(func() error {
		newProvider, err = suite.readConfigFromNode(nodeCtx)
		if err != nil {
			return retry.ExpectedError(err)
		}

		return nil
	}), "failed to read updated configuration from node %q: %w", node, err)

	suite.Assert().Equal(
		newProvider.Machine().Sysctls()[applyConfigTestSysctl],
		applyConfigTestSysctlVal,
	)
}

// TestApplyOnReboot verifies the apply config API without reboot.
func (suite *ApplyConfigSuite) TestApplyOnReboot() {
	suite.WaitForBootDone(suite.ctx)

	node := suite.RandomDiscoveredNode()
	suite.ClearConnectionRefused(suite.ctx, node)

	nodeCtx := client.WithNodes(suite.ctx, node)

	provider, err := suite.readConfigFromNode(nodeCtx)
	suite.Require().NoError(err, "failed to read existing config from node %q", node)

	cfg, ok := provider.(*v1alpha1.Config)
	suite.Require().True(ok)

	if cfg.MachineConfig.MachineSysctls == nil {
		cfg.MachineConfig.MachineSysctls = make(map[string]string)
	}

	cfg.MachineConfig.MachineSysctls[applyConfigNoRebootTestSysctl] = applyConfigNoRebootTestSysctlVal

	cfgDataOut, err := cfg.Bytes()
	suite.Require().NoError(err, "failed to marshal updated machine config data (node %q)", node)

	_, err = suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
		OnReboot: true,
		Data:     cfgDataOut,
	})
	suite.Require().NoError(err, "failed to apply deferred configuration (node %q): %w", node)

	// Verify configuration change
	var newProvider config.Provider

	newProvider, err = suite.readConfigFromNode(nodeCtx)

	suite.Require().NoError(err, "failed to read updated configuration from node %q: %w", node)

	suite.Assert().Equal(
		newProvider.Machine().Sysctls()[applyConfigNoRebootTestSysctl],
		applyConfigNoRebootTestSysctlVal,
	)

	cfg, ok = newProvider.(*v1alpha1.Config)
	suite.Require().True(ok)

	// revert back
	delete(cfg.MachineConfig.MachineSysctls, applyConfigNoRebootTestSysctl)

	cfgDataOut, err = cfg.Bytes()
	suite.Require().NoError(err, "failed to marshal updated machine config data (node %q)", node)

	_, err = suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
		OnReboot: true,
		Data:     cfgDataOut,
	})
	suite.Require().NoError(err, "failed to apply deferred configuration (node %q): %w", node)
}

// TestApplyConfigRotateEncryptionSecrets verify key rotation by sequential apply config calls.
func (suite *ApplyConfigSuite) TestApplyConfigRotateEncryptionSecrets() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	node := suite.RandomDiscoveredNode(machine.TypeJoin)
	suite.ClearConnectionRefused(suite.ctx, node)

	nodeCtx := client.WithNodes(suite.ctx, node)
	provider, err := suite.readConfigFromNode(nodeCtx)

	suite.Assert().NoError(err)

	encryption := provider.Machine().SystemDiskEncryption().Get(constants.EphemeralPartitionLabel)

	if encryption == nil {
		suite.T().Skip("skipped in not encrypted mode")
	}

	suite.WaitForBootDone(suite.ctx)

	cfg, ok := encryption.(*v1alpha1.EncryptionConfig)
	suite.Assert().True(ok)

	keySets := [][]*v1alpha1.EncryptionKey{
		{
			{
				KeyNodeID: &v1alpha1.EncryptionKeyNodeID{},
				KeySlot:   0,
			},
			{
				KeyStatic: &v1alpha1.EncryptionKeyStatic{
					KeyData: "AlO93jayutOpsDxDS=-",
				},
				KeySlot: 1,
			},
		},
		{
			{
				KeyStatic: &v1alpha1.EncryptionKeyStatic{
					KeyData: "AlO93jayutOpsDxDS=-",
				},
				KeySlot: 1,
			},
		},
		{
			{
				KeyNodeID: &v1alpha1.EncryptionKeyNodeID{},
				KeySlot:   0,
			},
			{
				KeyStatic: &v1alpha1.EncryptionKeyStatic{
					KeyData: "AlO93jayutOpsDxDS=-",
				},
				KeySlot: 1,
			},
		},
		{
			{
				KeyNodeID: &v1alpha1.EncryptionKeyNodeID{},
				KeySlot:   0,
			},
			{
				KeyStatic: &v1alpha1.EncryptionKeyStatic{
					KeyData: "1js4nfhvneJJsak=GVN4Inf5gh",
				},
				KeySlot: 1,
			},
		},
	}

	for _, keys := range keySets {
		cfg.EncryptionKeys = keys

		data, err := provider.Bytes()
		suite.Require().NoError(err)

		suite.AssertRebooted(suite.ctx, node, func(nodeCtx context.Context) error {
			_, err = suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
				Data: data,
			})
			if err != nil {
				// It is expected that the connection will EOF here, so just log the error
				suite.Assert().Nilf("failed to apply configuration (node %q): %w", node, err)
			}

			return nil
		}, assertRebootedRebootTimeout)

		suite.ClearConnectionRefused(suite.ctx, node)

		// Verify configuration change
		var newProvider config.Provider

		suite.Require().Nilf(retry.Constant(time.Minute, retry.WithUnits(time.Second)).Retry(func() error {
			newProvider, err = suite.readConfigFromNode(nodeCtx)
			if err != nil {
				return retry.ExpectedError(err)
			}

			return nil
		}), "failed to read updated configuration from node %q: %w", node, err)

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
	}
}

func (suite *ApplyConfigSuite) readConfigFromNode(nodeCtx context.Context) (config.Provider, error) {
	// Load the current node machine config
	cfgData := new(bytes.Buffer)

	reader, errCh, err := suite.Client.Read(nodeCtx, constants.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("error creating reader: %w", err)
	}
	defer reader.Close() //nolint:errcheck

	if err = copyFromReaderWithErrChan(cfgData, reader, errCh); err != nil {
		return nil, fmt.Errorf("error reading: %w", err)
	}

	provider, err := configloader.NewFromBytes(cfgData.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	return provider, nil
}

func copyFromReaderWithErrChan(out io.Writer, in io.Reader, errCh <-chan error) (err error) {
	var wg sync.WaitGroup

	var chanErr error

	wg.Add(1)

	go func() {
		defer wg.Done()

		// StreamReader is only singly-buffered, so we need to process any errors as we get them.
		for chanErr = range errCh {
		}
	}()

	defer func() {
		wg.Wait()

		if err == nil {
			err = chanErr
		}
	}()

	_, err = io.Copy(out, in)

	return err
}

func init() {
	allSuites = append(allSuites, new(ApplyConfigSuite))
}
