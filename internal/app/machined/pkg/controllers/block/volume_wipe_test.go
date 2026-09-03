// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	intmeta "github.com/siderolabs/talos/internal/pkg/meta"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type volumeWipeSuiteBase struct {
	ctest.DefaultSuite

	meta *intmeta.Meta
}

// setup registers the VolumeWipeController in the given v1alpha1 mode, backed by a temporary META.
func (suite *volumeWipeSuiteBase) setup(mode machineruntime.Mode) func(*ctest.DefaultSuite) {
	return func(s *ctest.DefaultSuite) {
		path := filepath.Join(s.T().TempDir(), "meta")

		f, err := os.Create(path)
		s.Require().NoError(err)
		s.Require().NoError(f.Truncate(1024 * 1024))
		s.Require().NoError(f.Close())

		// META keeps its own state, as on a real machine it is loaded before the controller runtime
		m, err := intmeta.New(s.Ctx(), state.WrapCore(namespaced.NewState(inmem.Build)), intmeta.WithFixedPath(path))
		s.Require().NoError(err)

		suite.meta = m

		s.Require().NoError(s.Runtime().RegisterController(
			&blockctrls.VolumeWipeController{
				V1Alpha1Mode: mode,
				MetaProvider: metaProvider{meta: m},
			},
		))
	}
}

// markMetaLoadedAndDevicesReady satisfies the two preconditions the controller waits for before
// looking at the staged wipe selectors.
func (suite *volumeWipeSuiteBase) markMetaLoadedAndDevicesReady() {
	metaLoaded := runtime.NewMetaLoaded()
	metaLoaded.TypedSpec().Done = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), metaLoaded))

	discoveredVolumesStatus := block.NewDiscoveredVolumesStatus(block.NamespaceName, block.DiscoveredVolumesStatusID)
	discoveredVolumesStatus.TypedSpec().Ready = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), discoveredVolumesStatus))
}

func (suite *volumeWipeSuiteBase) assertWipeStatusReady() {
	ctest.AssertResource(suite, block.VolumeWipeID, func(status *block.VolumeWipeStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Ready)
	})
}

// VolumeWipeContainerSuite covers container mode, where there is nothing to wipe.
type VolumeWipeContainerSuite struct {
	volumeWipeSuiteBase
}

func TestVolumeWipeContainerSuite(t *testing.T) {
	t.Parallel()

	s := &VolumeWipeContainerSuite{}
	s.DefaultSuite = ctest.DefaultSuite{
		Timeout:    10 * time.Second,
		AfterSetup: s.setup(machineruntime.ModeContainer),
	}

	suite.Run(t, s)
}

// TestWipeStatusReady verifies that the wipe status is published in container mode.
//
// A container has no block devices and no META: the container boot sequence never runs the
// reloadMeta task, so MetaLoaded is never created. If the controller waited for it,
// VolumeConfigController would never create a single non-META volume, and the boot would hang.
func (suite *VolumeWipeContainerSuite) TestWipeStatusReady() {
	suite.assertWipeStatusReady()
}

// VolumeWipeSuite covers the non-container modes.
type VolumeWipeSuite struct {
	volumeWipeSuiteBase
}

func TestVolumeWipeSuite(t *testing.T) {
	t.Parallel()

	s := &VolumeWipeSuite{}
	s.DefaultSuite = ctest.DefaultSuite{
		Timeout:    10 * time.Second,
		AfterSetup: s.setup(machineruntime.ModeMetal),
	}

	suite.Run(t, s)
}

// TestWaitsForPreconditions verifies that the wipe status is only published once META is loaded and
// the discovered volumes are ready: wiping a volume which hasn't been discovered yet would be a no-op.
func (suite *VolumeWipeSuite) TestWaitsForPreconditions() {
	ctest.AssertNoResource[*block.VolumeWipeStatus](suite, block.VolumeWipeID)

	suite.markMetaLoadedAndDevicesReady()

	// no staged wipe selectors: nothing to wipe
	suite.assertWipeStatusReady()
}

// TestSelectorMatchesNothing verifies that a staged wipe selector which matches no discovered volume
// doesn't block the boot, and that the staged wipe tag is consumed either way.
func (suite *VolumeWipeSuite) TestSelectorMatchesNothing() {
	selectors, err := json.Marshal([]cel.Expression{
		cel.MustExpression(cel.ParseBooleanExpression(`volume.partition_uuid == "3a0f1b2c-0000-0000-0000-000000000000"`, celenv.VolumeLocator())),
	})
	suite.Require().NoError(err)

	ok, err := suite.meta.SetTag(suite.Ctx(), meta.StagedWipeSelectors, string(selectors))
	suite.Require().NoError(err)
	suite.Require().True(ok)

	metaKey := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.StagedWipeSelectors))
	metaKey.TypedSpec().Value = string(selectors)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), metaKey))

	suite.markMetaLoadedAndDevicesReady()

	suite.assertWipeStatusReady()

	_, exists := suite.meta.ReadTag(meta.StagedWipeSelectors)
	suite.Assert().False(exists, "the staged wipe tag should be consumed even if a selector matches nothing")
}
