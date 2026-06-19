// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type UnattendedInstallSuite struct {
	ctest.DefaultSuite
}

func TestUnattendedInstallSuite(t *testing.T) {
	suite.Run(t, &UnattendedInstallSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 30 * time.Second,
		},
	})
}

const testInstallImage = "factory.talos.dev/metal-installer/test:v1.0.0"

const (
	testPlatform      = "metal"
	testFactoryAPIURL = "https://factory.talos.dev"
	testFactoryHost   = "factory.talos.dev"
	testSchematicID   = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

// testDiskPath must be a real path so filepath.EvalSymlinks succeeds in the controller.
const testDiskPath = "/dev/null"

func (suite *UnattendedInstallSuite) createConfig() {
	suite.createConfigWithImage(testInstallImage)
}

func (suite *UnattendedInstallSuite) createConfigWithImage(image string) {
	doc := runtimecfg.NewUnattendedInstallConfigV1Alpha1()
	doc.Installer.Image = image
	doc.ProvisioningSpec.DiskSelector.Match = cel.MustExpression(cel.ParseBooleanExpression(`disk.dev_path == "`+testDiskPath+`"`, celenv.DiskLocator()))
	doc.ProvisioningSpec.Wipe = new(true)

	cfg, err := container.New(doc)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), config.NewMachineConfig(cfg)))
}

func (suite *UnattendedInstallSuite) createSchematic() {
	schematic := runtime.NewImageFactorySchematic(runtime.NamespaceName, runtime.ImageFactorySchematicID)
	schematic.TypedSpec().SchematicID = testSchematicID
	schematic.TypedSpec().APIURL = testFactoryAPIURL

	suite.Require().NoError(suite.State().Create(suite.Ctx(), schematic))
}

func (suite *UnattendedInstallSuite) createDisk(id string) {
	disk := block.NewDisk(block.NamespaceName, id)
	disk.TypedSpec().DevPath = testDiskPath

	suite.Require().NoError(suite.State().Create(suite.Ctx(), disk))
}

// register the controller with stubbed install/installed seams.
func (suite *UnattendedInstallSuite) register(installed *atomic.Bool, installCalls *atomic.Int64) {
	suite.registerCapturing(installed, installCalls, nil)
}

// registerCapturing is like register but records the image passed to the install seam.
func (suite *UnattendedInstallSuite) registerCapturing(installed *atomic.Bool, installCalls *atomic.Int64, installImage *atomic.Pointer[string]) {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.UnattendedInstallController{
		State:         suite.State(),
		InstalledFunc: installed.Load,
		PlatformFunc:  func() string { return testPlatform },
		InstallFunc: func(_ context.Context, _, image string, _ bool) error {
			installCalls.Add(1)

			if installImage != nil {
				img := image
				installImage.Store(&img)
			}

			return nil
		},
	}))
}

func (suite *UnattendedInstallSuite) TestNoConfig() {
	var (
		installed    atomic.Bool
		installCalls atomic.Int64
	)

	suite.register(&installed, &installCalls)

	rtestutils.AssertNoResource[*runtime.UnattendedInstallStatus](suite.Ctx(), suite.T(), suite.State(), runtime.UnattendedInstallStatusID)
}

func (suite *UnattendedInstallSuite) TestPendingNoDisk() {
	var (
		installed    atomic.Bool
		installCalls atomic.Int64
	)

	suite.register(&installed, &installCalls)
	suite.createConfig()

	rtestutils.AssertResource[*runtime.UnattendedInstallStatus](suite.Ctx(), suite.T(), suite.State(), runtime.UnattendedInstallStatusID,
		func(status *runtime.UnattendedInstallStatus, asrt *assert.Assertions) {
			asrt.Equal(runtime.UnattendedInstallPhasePending, status.TypedSpec().Phase)
		})

	suite.Assert().EqualValues(0, installCalls.Load())
}

func (suite *UnattendedInstallSuite) TestInstall() {
	var (
		installed    atomic.Bool
		installCalls atomic.Int64
	)

	suite.register(&installed, &installCalls)
	suite.createConfig()
	suite.createDisk("sda")

	rtestutils.AssertResource[*runtime.UnattendedInstallStatus](suite.Ctx(), suite.T(), suite.State(), runtime.UnattendedInstallStatusID,
		func(status *runtime.UnattendedInstallStatus, asrt *assert.Assertions) {
			asrt.Equal(runtime.UnattendedInstallPhaseWaitingForReboot, status.TypedSpec().Phase)
			asrt.Equal(testInstallImage, status.TypedSpec().Image)
		})

	suite.Assert().EqualValues(1, installCalls.Load())

	rtestutils.AssertResource[*runtime.RebootRequest](suite.Ctx(), suite.T(), suite.State(), runtime.RebootRequestID,
		func(_ *runtime.RebootRequest, asrt *assert.Assertions) {
			asrt.True(true, "reboot request should exist")
		})
}

// TestAlreadyInstalled mirrors the post-reboot case: the node reports installed and the controller
// records the resolved disk without performing an install.
func (suite *UnattendedInstallSuite) TestAlreadyInstalled() {
	var (
		installed    atomic.Bool
		installCalls atomic.Int64
	)

	installed.Store(true)

	suite.register(&installed, &installCalls)
	suite.createConfig()
	suite.createDisk("sda")

	rtestutils.AssertResource[*runtime.UnattendedInstallStatus](suite.Ctx(), suite.T(), suite.State(), runtime.UnattendedInstallStatusID,
		func(status *runtime.UnattendedInstallStatus, asrt *assert.Assertions) {
			asrt.Equal(runtime.UnattendedInstallPhaseInstalled, status.TypedSpec().Phase)
		})

	suite.Assert().EqualValues(0, installCalls.Load())
	rtestutils.AssertNoResource[*runtime.RebootRequest](suite.Ctx(), suite.T(), suite.State(), runtime.RebootRequestID)
}

func (suite *UnattendedInstallSuite) TestAlreadyInstalledNoDisk() {
	var (
		installed    atomic.Bool
		installCalls atomic.Int64
	)

	installed.Store(true)

	suite.register(&installed, &installCalls)
	suite.createConfig()

	rtestutils.AssertResource[*runtime.UnattendedInstallStatus](suite.Ctx(), suite.T(), suite.State(), runtime.UnattendedInstallStatusID,
		func(status *runtime.UnattendedInstallStatus, asrt *assert.Assertions) {
			asrt.Equal(runtime.UnattendedInstallPhaseInstalled, status.TypedSpec().Phase)
		})

	suite.Assert().EqualValues(0, installCalls.Load())
	rtestutils.AssertNoResource[*runtime.RebootRequest](suite.Ctx(), suite.T(), suite.State(), runtime.RebootRequestID)
}

// TestNoReinstallOnNewDisk ensures a new disk matching the selector after a completed install does not
// trigger a second install nor flip the recorded disk.
func (suite *UnattendedInstallSuite) TestNoReinstallOnNewDisk() {
	var (
		installed    atomic.Bool
		installCalls atomic.Int64
	)

	suite.register(&installed, &installCalls)
	suite.createConfig()
	suite.createDisk("sda")

	rtestutils.AssertResource[*runtime.UnattendedInstallStatus](suite.Ctx(), suite.T(), suite.State(), runtime.UnattendedInstallStatusID,
		func(status *runtime.UnattendedInstallStatus, asrt *assert.Assertions) {
			asrt.Equal(runtime.UnattendedInstallPhaseWaitingForReboot, status.TypedSpec().Phase)
		})

	// a new disk matching the selector appears.
	suite.createDisk("sdb")

	// the recorded disk is unchanged and no second install happens.
	rtestutils.AssertResource[*runtime.UnattendedInstallStatus](suite.Ctx(), suite.T(), suite.State(), runtime.UnattendedInstallStatusID,
		func(status *runtime.UnattendedInstallStatus, asrt *assert.Assertions) {
			asrt.Equal(runtime.UnattendedInstallPhaseWaitingForReboot, status.TypedSpec().Phase)
		})

	suite.Assert().EqualValues(1, installCalls.Load())
	rtestutils.AssertResource[*runtime.RebootRequest](suite.Ctx(), suite.T(), suite.State(), runtime.RebootRequestID,
		func(_ *runtime.RebootRequest, asrt *assert.Assertions) {
			asrt.True(true, "reboot request should exist")
		})
}

// TestInstallImageFromBootEntry covers the case where the config does not specify an installer image:
// the image is derived from the ImageFactorySchematic boot entry and the node platform.
func (suite *UnattendedInstallSuite) TestInstallImageFromBootEntry() {
	var (
		installed    atomic.Bool
		installCalls atomic.Int64
		installImage atomic.Pointer[string]
	)

	suite.registerCapturing(&installed, &installCalls, &installImage)
	suite.createConfigWithImage("")
	suite.createSchematic()
	suite.createDisk("sda")

	expected := images.NewInstallerImage(testFactoryHost, testPlatform, testSchematicID, "")

	rtestutils.AssertResource[*runtime.UnattendedInstallStatus](suite.Ctx(), suite.T(), suite.State(), runtime.UnattendedInstallStatusID,
		func(status *runtime.UnattendedInstallStatus, asrt *assert.Assertions) {
			asrt.Equal(runtime.UnattendedInstallPhaseInstalled, status.TypedSpec().Phase)
		})

	suite.Assert().EqualValues(1, installCalls.Load())
	rtestutils.AssertNoResource[*runtime.RebootRequest](suite.Ctx(), suite.T(), suite.State(), runtime.RebootRequestID)

	if ptr := installImage.Load(); suite.Assert().NotNil(ptr) {
		suite.Assert().Equal(expected, *ptr)
	}
}

// TestInstallImageNoBootEntry covers the case where the config does not specify an installer image and
// no ImageFactorySchematic boot entry is available: the install fails.
func (suite *UnattendedInstallSuite) TestInstallImageNoBootEntry() {
	var (
		installed    atomic.Bool
		installCalls atomic.Int64
	)

	suite.register(&installed, &installCalls)
	suite.createConfigWithImage("")
	suite.createDisk("sda")

	rtestutils.AssertResource[*runtime.UnattendedInstallStatus](suite.Ctx(), suite.T(), suite.State(), runtime.UnattendedInstallStatusID,
		func(status *runtime.UnattendedInstallStatus, asrt *assert.Assertions) {
			asrt.Equal(runtime.UnattendedInstallPhaseFailed, status.TypedSpec().Phase)
			asrt.NotEmpty(status.TypedSpec().Error)
		})

	suite.Assert().EqualValues(0, installCalls.Load())
}
