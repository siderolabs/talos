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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type PlatformConfigLoadSuite struct {
	ctest.DefaultSuite
}

func (suite *PlatformConfigLoadSuite) TestLoadConfig() {
	statePath := suite.T().TempDir()
	mountID := (&netctrl.PlatformConfigLoadController{}).Name() + "-" + constants.StatePartitionLabel

	suite.Require().NoError(
		os.WriteFile(
			filepath.Join(statePath, constants.PlatformNetworkConfigFilename),
			[]byte(sampleStoredConfig),
			0o400,
		),
	)

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})

	volumeMountStatus := block.NewVolumeMountStatus(block.NamespaceName, mountID)
	volumeMountStatus.TypedSpec().Target = statePath
	suite.Create(volumeMountStatus)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)

	suite.Destroy(volumeMountStatus)

	ctest.AssertResource(suite, network.PlatformConfigCachedID, func(cachedConfig *network.PlatformConfig, asrt *assert.Assertions) {
		asrt.Equal(
			[]network.HostnameSpecSpec{
				{
					Hostname: "talos-e2e-897b4e49-gcp-controlplane-jvcnl",
				},
			},
			cachedConfig.TypedSpec().Hostnames,
		)
		asrt.Equal(
			[]netip.Addr{
				netip.MustParseAddr("10.3.4.5"),
				netip.MustParseAddr("2001:470:6d:30e:96f4:4219:5733:b860"),
			},
			cachedConfig.TypedSpec().ExternalIPs,
		)
	})
}

func TestPlatformConfigLoadSuite(t *testing.T) {
	t.Parallel()

	if os.Geteuid() != 0 {
		t.Skip("skipping test that requires root privileges")
	}

	suite.Run(t, &PlatformConfigLoadSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(
					suite.Runtime().RegisterController(&netctrl.PlatformConfigLoadController{}),
				)
			},
		},
	})
}
