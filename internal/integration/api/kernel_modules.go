// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bufio"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// KernelModulesSuite verifies on-demand kernel module loading via the modprobe usermode helper.
type KernelModulesSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *KernelModulesSuite) SuiteName() string {
	return "api.KernelModulesSuite"
}

// SetupTest ...
func (suite *KernelModulesSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)

	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Skip("skipping kernel test since Talos kernel is not running")
	}

	if suite.SelinuxEnforcing {
		suite.T().Skip("skipping in SELinux enforcing mode: host namespace debug mode doesn't function")
	}
}

// TearDownTest ...
func (suite *KernelModulesSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestLoadedOnDemand verifies that a module requested by the kernel by name is loaded without being listed in the machine config.
//
// Device-mapper resolves the 'thin-pool' target type via request_module("dm-thin-pool").
func (suite *KernelModulesSuite) TestLoadedOnDemand() {
	const (
		module         = "dm-thin-pool"
		moduleStatusID = "dm_thin_pool"
	)

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("using node %s", node)

	suite.skipIfModuleLoaded(node, moduleStatusID)

	loadedBefore := suite.countModuleLoadedLogLines(node, module)

	// device-mapper resolves the target type (which loads the module) before parsing the table,
	// so the table is deliberately invalid: /dev/null is not a block device, and no device is left behind
	out, exitCode := suite.RunDebugContainer(suite.ctx, node, "sh", "-c",
		"dmsetup create talos-test-thin-pool --table '0 8 thin-pool /dev/null /dev/null 128 0'; dmsetup remove talos-test-thin-pool 2>/dev/null; true",
	)

	suite.T().Logf("dmsetup exit code %d, output:\n%s", exitCode, out)

	// the module gets loaded on demand
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, moduleStatusID,
		func(*runtime.KernelModuleStatus, *assert.Assertions) {},
	)

	suite.Assert().Eventually(func() bool {
		return suite.countModuleLoadedLogLines(node, module) > loadedBefore
	}, 30*time.Second, time.Second, "expected the modprobe helper to log the module load")
}

// TestAliasRefused verifies that a module requested via a subsystem alias (as userspace can trigger) is not loaded.
//
// Mounting a filesystem of an unknown type makes the kernel call request_module("fs-btrfs"), 'btrfs' being a module shipped with Talos.
func (suite *KernelModulesSuite) TestAliasRefused() {
	if testing.Short() {
		suite.T().Skip("skipping test in short mode")
	}

	const (
		module         = "btrfs"
		moduleStatusID = "btrfs"
	)

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("using node %s", node)

	suite.skipIfModuleLoaded(node, moduleStatusID)

	loadedBefore := suite.countModuleLoadedLogLines(node, module)

	// the filesystem type is resolved (which would load the module via the alias) before the source is checked
	out, exitCode := suite.RunDebugContainer(suite.ctx, node, "sh", "-c",
		"busybox mount -t btrfs /dev/null /tmp; true",
	)

	suite.T().Logf("mount exit code %d, output:\n%s", exitCode, out)

	// give the request a chance to be (wrongly) handled
	time.Sleep(10 * time.Second)

	rtestutils.AssertNoResource[*runtime.KernelModuleStatus](nodeCtx, suite.T(), suite.Client.COSI, moduleStatusID)

	suite.Assert().Equal(loadedBefore, suite.countModuleLoadedLogLines(node, module), "the alias request should not load the module")
}

// skipIfModuleLoaded skips the test if the module is already loaded, as Talos never unloads modules.
func (suite *KernelModulesSuite) skipIfModuleLoaded(node, moduleStatusID string) {
	nodeCtx := client.WithNode(suite.ctx, node)

	_, err := safe.StateGetByID[*runtime.KernelModuleStatus](nodeCtx, suite.Client.COSI, moduleStatusID)
	if err == nil {
		suite.T().Skipf("skipping test, module %q is already loaded on node %s", moduleStatusID, node)
	}

	if !state.IsNotFoundError(err) {
		suite.Require().NoError(err)
	}
}

// countModuleLoadedLogLines counts the kernel log lines of the modprobe helper about loading the module.
func (suite *KernelModulesSuite) countModuleLoadedLogLines(node, module string) int {
	nodeCtx := client.WithNode(suite.ctx, node)

	dmesgStream, err := suite.Client.Dmesg(nodeCtx, false, false)
	suite.Require().NoError(err)

	logReader, err := client.ReadStream(dmesgStream)
	suite.Require().NoError(err)

	defer logReader.Close() //nolint:errcheck

	var count int

	scanner := bufio.NewScanner(logReader)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "modprobe") && strings.Contains(line, `module "`+module+`" loaded`) {
			count++
		}
	}

	suite.Require().NoError(scanner.Err())

	return count
}

func init() {
	allSuites = append(allSuites, new(KernelModulesSuite))
}
