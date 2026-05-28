// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/smart"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type mockCollector struct {
	results map[string]smart.Result
}

func (m *mockCollector) Collect(devPath string) smart.Result {
	if r, ok := m.results[devPath]; ok {
		return r
	}

	return smart.Result{
		Source: block.DiskHealthSourceUnsupported,
		Status: block.DiskHealthStatusValueUnknown,
		Error:  "no supported disk health collector for this device",
	}
}

type DiskHealthStatusSuite struct {
	ctest.DefaultSuite
}

func TestDiskHealthStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &DiskHealthStatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				mock := &mockCollector{
					results: map[string]smart.Result{
						"/dev/nvme0n1": {
							Source:             block.DiskHealthSourceNVMe,
							Status:             block.DiskHealthStatusValueHealthy,
							TemperatureCelsius: 42,
							PowerOnHours:       12345,
							PowerCycles:        27,
							NVMe: &block.DiskHealthNVMeDetails{
								CriticalWarning:             0,
								PercentageUsed:              12,
								UnsafeShutdowns:             3,
								MediaAndDataIntegrityErrors: 0,
							},
						},
						"/dev/sda": {
							Source:             block.DiskHealthSourceATA,
							Status:             block.DiskHealthStatusValueWarning,
							TemperatureCelsius: 38,
							PowerOnHours:       50000,
							PowerCycles:        100,
							ATA: &block.DiskHealthATADetails{
								ReallocatedSectorCount: 5,
							},
						},
					},
				}

				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.DiskHealthStatusController{
					Collector: mock,
				}))
			},
		},
	})
}

func (suite *DiskHealthStatusSuite) TestNVMeDisk() {
	disk := block.NewDisk(block.NamespaceName, "nvme0n1")
	disk.TypedSpec().DevPath = "/dev/nvme0n1"
	disk.TypedSpec().Transport = "nvme"
	suite.Create(disk)

	ctest.AssertResource(suite, "nvme0n1", func(dhs *block.DiskHealthStatus, asrt *assert.Assertions) {
		asrt.Equal("nvme0n1", dhs.TypedSpec().DiskID)
		asrt.Equal("/dev/nvme0n1", dhs.TypedSpec().Device)
		asrt.Equal(block.DiskHealthSourceNVMe, dhs.TypedSpec().HealthSource)
		asrt.Equal(block.DiskHealthStatusValueHealthy, dhs.TypedSpec().Status)
		asrt.EqualValues(42, dhs.TypedSpec().TemperatureCelsius)
		asrt.EqualValues(12345, dhs.TypedSpec().PowerOnHours)
		asrt.EqualValues(27, dhs.TypedSpec().PowerCycles)
		asrt.NotNil(dhs.TypedSpec().Details.NVMe)
		asrt.EqualValues(12, dhs.TypedSpec().Details.NVMe.PercentageUsed)
		asrt.EqualValues(3, dhs.TypedSpec().Details.NVMe.UnsafeShutdowns)
	})
}

func (suite *DiskHealthStatusSuite) TestATADisk() {
	disk := block.NewDisk(block.NamespaceName, "sda")
	disk.TypedSpec().DevPath = "/dev/sda"
	disk.TypedSpec().Transport = "ata"
	suite.Create(disk)

	ctest.AssertResource(suite, "sda", func(dhs *block.DiskHealthStatus, asrt *assert.Assertions) {
		asrt.Equal("sda", dhs.TypedSpec().DiskID)
		asrt.Equal("/dev/sda", dhs.TypedSpec().Device)
		asrt.Equal(block.DiskHealthSourceATA, dhs.TypedSpec().HealthSource)
		asrt.Equal(block.DiskHealthStatusValueWarning, dhs.TypedSpec().Status)
		asrt.EqualValues(38, dhs.TypedSpec().TemperatureCelsius)
		asrt.NotNil(dhs.TypedSpec().Details.ATA)
		asrt.EqualValues(5, dhs.TypedSpec().Details.ATA.ReallocatedSectorCount)
	})
}

func (suite *DiskHealthStatusSuite) TestUnsupportedDisk() {
	disk := block.NewDisk(block.NamespaceName, "sdb")
	disk.TypedSpec().DevPath = "/dev/sdb"
	disk.TypedSpec().Transport = "usb"
	suite.Create(disk)

	ctest.AssertResource(suite, "sdb", func(dhs *block.DiskHealthStatus, asrt *assert.Assertions) {
		asrt.Equal("sdb", dhs.TypedSpec().DiskID)
		asrt.Equal(block.DiskHealthSourceUnsupported, dhs.TypedSpec().HealthSource)
		asrt.Equal(block.DiskHealthStatusValueUnknown, dhs.TypedSpec().Status)
		asrt.NotEmpty(dhs.TypedSpec().Error)
	})
}

func (suite *DiskHealthStatusSuite) TestDiskRemoval() {
	disk := block.NewDisk(block.NamespaceName, "nvme0n1")
	disk.TypedSpec().DevPath = "/dev/nvme0n1"
	disk.TypedSpec().Transport = "nvme"
	suite.Create(disk)

	ctest.AssertResource(suite, "nvme0n1", func(dhs *block.DiskHealthStatus, asrt *assert.Assertions) {
		asrt.Equal(block.DiskHealthSourceNVMe, dhs.TypedSpec().HealthSource)
	})

	suite.Destroy(disk)

	ctest.AssertNoResource[*block.DiskHealthStatus](suite, "nvme0n1")
}
