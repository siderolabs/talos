// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// UdevSuite verifies custom udev rule reconciliation.
type UdevSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *UdevSuite) SuiteName() string {
	return "api.UdevSuite"
}

// SetupTest ...
func (suite *UdevSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 2*time.Minute)
}

// TearDownTest ...
func (suite *UdevSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestUdevRulesConfig verifies that custom udev rules are written, udevd is reloaded,
// and existing devices are retriggered.
func (suite *UdevSuite) TestUdevRulesConfig() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing on node %q", node)

	diskID, devPath := suite.pickUdevTestDisk(nodeCtx)
	if diskID == "" {
		suite.T().Skipf("no suitable block disk found on node %q", node)
	}

	symlink := "/dev/talos-udev-test-" + diskID

	cfg := runtimecfg.NewUdevRulesConfigV1Alpha1()
	cfg.UdevRules = []string{
		fmt.Sprintf(`SUBSYSTEM=="block", KERNEL=="%s", SYMLINK+="%s"`, diskID, strings.TrimPrefix(symlink, "/dev/")),
	}

	suite.PatchMachineConfig(nodeCtx, cfg)
	defer suite.RemoveMachineConfigDocuments(nodeCtx, runtimecfg.UdevRulesConfigKind)

	// Check /dev directly: block.Disk symlinks are not guaranteed to refresh when
	// udev adds an alias for an already discovered device.
	suite.EventuallyWithT(
		func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			asrt.True(suite.udevSymlinkExists(nodeCtx, symlink), "expected %q to exist", symlink)
		},
		time.Minute, time.Second,
		"waiting for udev rule to create symlink %q for %s", symlink, devPath,
	)

	rules := suite.ReadFile(nodeCtx, "/usr/lib/udev/rules.d/99-talos.rules")
	suite.Require().Contains(rules, strings.TrimPrefix(symlink, "/dev/"))
}

func (suite *UdevSuite) pickUdevTestDisk(nodeCtx context.Context) (string, string) {
	disks, err := safe.StateListAll[*block.Disk](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	for disk := range disks.All() {
		spec := disk.TypedSpec()
		if spec.Readonly || spec.CDROM || spec.DevPath == "" {
			continue
		}

		diskID := filepath.Base(spec.DevPath)
		if diskID == "." || diskID == string(filepath.Separator) {
			continue
		}

		if slices.ContainsFunc(spec.Symlinks, func(symlink string) bool {
			return strings.HasPrefix(symlink, "/dev/talos-udev-test-")
		}) {
			continue
		}

		return diskID, spec.DevPath
	}

	return "", ""
}

func (suite *UdevSuite) udevSymlinkExists(nodeCtx context.Context, symlink string) bool {
	stream, err := suite.Client.LS(nodeCtx, &machineapi.ListRequest{
		Root:  "/dev",
		Types: []machineapi.ListRequest_Type{machineapi.ListRequest_SYMLINK},
	})
	if err != nil {
		return false
	}

	var found bool

	if err = helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, node string, multipleNodes bool) error {
		if info.Name == symlink || info.GetRelativeName() == filepath.Base(symlink) {
			found = true
		}

		return nil
	}); err != nil {
		return false
	}

	return found
}

func init() {
	allSuites = append(allSuites, new(UdevSuite))
}
