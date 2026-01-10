// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type PlatformConfigStoreSuite struct {
	ctest.DefaultSuite
}

const sampleStoredConfig = "addresses: []\nlinks: []\nroutes: []\nhostnames:\n    - hostname: talos-e2e-897b4e49-gcp-controlplane-jvcnl\n      domainname: \"\"\n      layer: default\nresolvers: []\ntimeServers: []\noperators: []\nexternalIPs:\n    - 10.3.4.5\n    - 2001:470:6d:30e:96f4:4219:5733:b860\n" //nolint:lll

func (suite *PlatformConfigStoreSuite) TestStoreConfig() {
	platformConfig := network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigActiveID)
	platformConfig.TypedSpec().Hostnames = []network.HostnameSpecSpec{
		{
			Hostname: "talos-e2e-897b4e49-gcp-controlplane-jvcnl",
		},
	}
	platformConfig.TypedSpec().ExternalIPs = []netip.Addr{
		netip.MustParseAddr("10.3.4.5"),
		netip.MustParseAddr("2001:470:6d:30e:96f4:4219:5733:b860"),
	}
	suite.Create(platformConfig)

	statePath := suite.T().TempDir()
	mountID := (&netctrl.PlatformConfigStoreController{}).Name() + "-" + constants.StatePartitionLabel

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})

	volumeMountStatus := block.NewVolumeMountStatus(mountID)
	volumeMountStatus.TypedSpec().Target = statePath
	suite.Create(volumeMountStatus)

	suite.EventuallyWithT(func(collect *assert.CollectT) {
		asrt := assert.New(collect)

		contents, err := os.ReadFile(filepath.Join(statePath, constants.PlatformNetworkConfigFilename))
		asrt.NoError(err)

		asrt.Equal(sampleStoredConfig, string(contents))
	}, time.Second, 10*time.Millisecond)

	ctest.AssertResources(suite, []resource.ID{volumeMountStatus.Metadata().ID()}, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(vms.Metadata().Finalizers().Empty())
	})

	suite.Destroy(volumeMountStatus)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)

	// do an update which should not trigger store operation
	platformConfig.Metadata().Labels().Set("foo", "bar")
	suite.Update(platformConfig)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)

	// now update configuration
	platformConfig.TypedSpec().Hostnames = nil
	suite.Update(platformConfig)

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})
}

func TestPlatformConfigStoreSuite(t *testing.T) {
	t.Parallel()

	if os.Geteuid() != 0 {
		t.Skip("skipping test that requires root privileges")
	}

	suite.Run(t, &PlatformConfigStoreSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(
					suite.Runtime().RegisterController(&netctrl.PlatformConfigStoreController{}),
				)
			},
		},
	})
}
