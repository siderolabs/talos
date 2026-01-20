// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	configctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/config"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

type PersistenceSuite struct {
	ctest.DefaultSuite

	cfg1, cfg2 talosconfig.Provider
}

func (suite *PersistenceSuite) TestPersist() {
	volumeLifecycle := block.NewVolumeLifecycle(block.NamespaceName, block.VolumeLifecycleID)
	suite.Create(volumeLifecycle)

	ctest.AssertResource(suite, block.VolumeLifecycleID, func(vl *block.VolumeLifecycle, asrt *assert.Assertions) {
		asrt.False(vl.Metadata().Finalizers().Empty())
	})

	statePath := suite.T().TempDir()
	mountID := (&configctrl.PersistenceController{}).Name() + "-" + constants.StatePartitionLabel

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)

	c1 := config.NewMachineConfigWithID(suite.cfg1, config.PersistentID)
	suite.Create(c1)

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})

	volumeMountStatus := block.NewVolumeMountStatus(mountID)
	volumeMountStatus.TypedSpec().Target = statePath
	suite.Create(volumeMountStatus)

	suite.EventuallyWithT(func(collect *assert.CollectT) {
		asrt := assert.New(collect)

		asrt.FileExists(filepath.Join(statePath, constants.ConfigFilename))
	}, time.Second, 10*time.Millisecond)

	ctest.AssertResources(suite, []resource.ID{volumeMountStatus.Metadata().ID()}, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(vms.Metadata().Finalizers().Empty())
	})

	suite.Destroy(volumeMountStatus)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)

	c2 := config.NewMachineConfigWithID(suite.cfg2, config.PersistentID)
	c2.Metadata().SetVersion(c1.Metadata().Version())
	suite.Update(c2)

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})

	// teardown the volume lifecycle, but finalizer should not be removed yet
	_, err := suite.State().Teardown(suite.Ctx(), volumeLifecycle.Metadata())
	suite.Require().NoError(err)

	ctest.AssertResource(suite, block.VolumeLifecycleID, func(vl *block.VolumeLifecycle, asrt *assert.Assertions) {
		asrt.False(vl.Metadata().Finalizers().Empty())
	})

	volumeMountStatus = block.NewVolumeMountStatus(mountID)
	volumeMountStatus.TypedSpec().Target = statePath
	suite.Create(volumeMountStatus)

	suite.EventuallyWithT(func(collect *assert.CollectT) {
		asrt := assert.New(collect)

		contents, err := os.ReadFile(filepath.Join(statePath, constants.ConfigFilename))
		asrt.NoError(err)

		asrt.Contains(string(contents), "jointoken=none")
	}, time.Second, 10*time.Millisecond)

	ctest.AssertResources(suite, []resource.ID{volumeMountStatus.Metadata().ID()}, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(vms.Metadata().Finalizers().Empty())
	})

	suite.Destroy(volumeMountStatus)

	ctest.AssertResource(suite, block.VolumeLifecycleID, func(vl *block.VolumeLifecycle, asrt *assert.Assertions) {
		asrt.True(vl.Metadata().Finalizers().Empty())
	})

	suite.Destroy(volumeLifecycle)
}

func (suite *PersistenceSuite) TestConfig() {
	volumeLifecycle := block.NewVolumeLifecycle(block.NamespaceName, block.VolumeLifecycleID)
	suite.Create(volumeLifecycle)

	ctest.AssertResource(suite, block.VolumeLifecycleID, func(vl *block.VolumeLifecycle, asrt *assert.Assertions) {
		asrt.False(vl.Metadata().Finalizers().Empty())
	})

	_, err := suite.State().Teardown(suite.Ctx(), volumeLifecycle.Metadata())
	suite.Require().NoError(err)

	ctest.AssertResource(suite, block.VolumeLifecycleID, func(vl *block.VolumeLifecycle, asrt *assert.Assertions) {
		asrt.True(vl.Metadata().Finalizers().Empty())
	})

	suite.Destroy(volumeLifecycle)
}

func TestPersistenceSuite(t *testing.T) {
	t.Parallel()

	if os.Geteuid() != 0 {
		t.Skip("skipping test that requires root privileges")
	}

	sideroLinkCfg1 := siderolink.NewConfigV1Alpha1()
	sideroLinkCfg1.APIUrlConfig.URL = must(url.Parse("https://siderolink.api/?jointoken=secret&user=alice"))

	cfg1, err := container.New(sideroLinkCfg1)
	require.NoError(t, err)

	sideroLinkCfg2 := siderolink.NewConfigV1Alpha1()
	sideroLinkCfg2.APIUrlConfig.URL = must(url.Parse("https://siderolink.api/?jointoken=none&user=bob"))

	cfg2, err := container.New(sideroLinkCfg2)
	require.NoError(t, err)

	suite.Run(t, &PersistenceSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&configctrl.PersistenceController{}))
			},
		},

		cfg1: cfg1,
		cfg2: cfg2,
	})
}
