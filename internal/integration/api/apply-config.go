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

	"github.com/talos-systems/talos/internal/integration/base"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
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

const assertRebootedRebootTimeout = 10 * time.Minute

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

// TestRecoverControlPlane removes the control plane components and attempts to recover them with the recover API.
func (suite *ApplyConfigSuite) TestRecoverControlPlane() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot")
	}

	nodes := suite.DiscoverNodes().NodesByType(machine.TypeJoin)
	suite.Require().NotEmpty(nodes)

	sort.Strings(nodes)

	node := nodes[0]

	nodeCtx := client.WithNodes(suite.ctx, node)

	provider, err := suite.readConfigFromNode(nodeCtx)
	suite.Assert().Nilf(err, "failed to read existing config from node %q: %w", node, err)

	provider.Machine().Sysctls()[applyConfigTestSysctl] = applyConfigTestSysctlVal

	cfgDataOut, err := provider.Bytes()
	suite.Assert().Nilf(err, "failed to marshal updated machine config data (node %q): %w", node, err)

	suite.AssertRebooted(suite.ctx, node, func(nodeCtx context.Context) error {
		_, err := suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
			Data: cfgDataOut,
		})
		if err != nil {
			// It is expected that the connection will EOF here, so just log the error
			suite.Assert().Nilf("failed to apply configuration (node %q): %w", node, err)
		}

		return nil
	}, assertRebootedRebootTimeout)

	// Verify configuration change
	newProvider, err := suite.readConfigFromNode(nodeCtx)
	suite.Assert().Nilf(err, "failed to read updated configuration from node %q: %w", node, err)

	suite.Assert().Equal(
		newProvider.Machine().Sysctls()[applyConfigTestSysctl],
		applyConfigTestSysctlVal,
	)
}

func (suite *ApplyConfigSuite) readConfigFromNode(nodeCtx context.Context) (config.Provider, error) {
	// Load the current node machine config
	cfgData := new(bytes.Buffer)

	reader, errCh, err := suite.Client.Read(nodeCtx, constants.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("error creating reader: %w", err)
	}
	defer reader.Close()

	if err := copyFromReaderWithErrChan(cfgData, reader, errCh); err != nil {
		return nil, fmt.Errorf("error reading: %w", err)
	}

	provider, err := configloader.NewFromBytes(cfgData.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	return provider, nil
}

func copyFromReaderWithErrChan(out io.Writer, in io.Reader, errCh <-chan error) error {
	var wg sync.WaitGroup

	var chanErr error

	wg.Add(1)
	go func() {
		defer wg.Done()

		// StreamReader is only singly-buffered, so we need to process any errors as we get them.
		for chanErr = range errCh {
		}
	}()

	defer wg.Wait()

	_, err := io.Copy(out, in)
	if err != nil {
		return err
	}

	if chanErr != nil {
		return chanErr
	}

	return nil
}

func init() {
	allSuites = append(allSuites, new(ApplyConfigSuite))
}
