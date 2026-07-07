// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// fakeProber returns canned SMART data keyed by device path, so the controller
// can be exercised without real hardware.
type fakeProber struct{}

func (fakeProber) Probe(devPath string, rotational bool) (block.SMARTStatusSpec, bool, error) {
	switch devPath {
	case "/dev/nvme0n1":
		return block.SMARTStatusSpec{
			DevPath:     devPath,
			DevType:     "nvme",
			Healthy:     true,
			Temperature: 40,
			PercentUsed: 3,
		}, false, nil
	case "/dev/sda": // rotational disk which is spun down
		return block.SMARTStatusSpec{
			DevPath:    devPath,
			DevType:    "sata",
			PowerState: "standby",
		}, true, nil
	default: // disk which does not support SMART
		return block.SMARTStatusSpec{}, false, errors.New("device does not support SMART")
	}
}

type SMARTStatusSuite struct {
	ctest.DefaultSuite
}

func TestSMARTStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &SMARTStatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
		},
	})
}

// smartMachineConfig builds a machine config holding a single DiskSMARTConfig document, which
// is what enables SMART collection.
func smartMachineConfig(suite *SMARTStatusSuite) *config.MachineConfig {
	ctr, err := container.New(blockcfg.NewDiskSMARTConfigV1Alpha1())
	suite.Require().NoError(err)

	return config.NewMachineConfig(ctr)
}

// emptyMachineConfig builds a machine config without a DiskSMARTConfig document, which must keep
// SMART collection off.
func emptyMachineConfig(suite *SMARTStatusSuite) *config.MachineConfig {
	ctr, err := container.New()
	suite.Require().NoError(err)

	return config.NewMachineConfig(ctr)
}

func (suite *SMARTStatusSuite) TestReconcile() {
	suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.SMARTStatusController{
		Prober: fakeProber{},
	}))

	// SMART is collected because the DiskSMARTConfig document is present.
	suite.Create(smartMachineConfig(suite))

	// an NVMe disk which reports SMART data
	nvme := block.NewDisk(block.NamespaceName, "nvme0n1")
	nvme.TypedSpec().DevPath = "/dev/nvme0n1"
	suite.Create(nvme)

	// a rotational SATA disk which is in standby
	sda := block.NewDisk(block.NamespaceName, "sda")
	sda.TypedSpec().DevPath = "/dev/sda"
	sda.TypedSpec().Rotational = true
	suite.Create(sda)

	// a CD-ROM which must be skipped
	sr0 := block.NewDisk(block.NamespaceName, "sr0")
	sr0.TypedSpec().DevPath = "/dev/sr0"
	sr0.TypedSpec().CDROM = true
	suite.Create(sr0)

	// a disk which does not support SMART
	vda := block.NewDisk(block.NamespaceName, "vda")
	vda.TypedSpec().DevPath = "/dev/vda"
	suite.Create(vda)

	ctest.AssertResource(suite, "nvme0n1", func(s *block.SMARTStatus, asrt *assert.Assertions) {
		asrt.Equal("/dev/nvme0n1", s.TypedSpec().DevPath)
		asrt.Equal("nvme", s.TypedSpec().DevType)
		asrt.True(s.TypedSpec().Healthy)
		asrt.EqualValues(40, s.TypedSpec().Temperature)
		asrt.EqualValues(3, s.TypedSpec().PercentUsed)
	})

	ctest.AssertResource(suite, "sda", func(s *block.SMARTStatus, asrt *assert.Assertions) {
		asrt.Equal("sata", s.TypedSpec().DevType)
		asrt.Equal("standby", s.TypedSpec().PowerState)
		asrt.Equal("skipped: disk in standby", s.TypedSpec().Message)
		// a freshly-observed standby disk is reported healthy until it can be probed.
		asrt.True(s.TypedSpec().Healthy)
	})

	// no SMART status for the CD-ROM or the disk which does not support SMART.
	ctest.AssertNoResource[*block.SMARTStatus](suite, "sr0")
	ctest.AssertNoResource[*block.SMARTStatus](suite, "vda")
}

// TestDisabledWithoutDocument asserts that SMART is collected neither without a machine config,
// nor with a machine config which carries no DiskSMARTConfig document.
func (suite *SMARTStatusSuite) TestDisabledWithoutDocument() {
	suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.SMARTStatusController{
		Prober: fakeProber{},
	}))

	nvme := block.NewDisk(block.NamespaceName, "nvme0n1")
	nvme.TypedSpec().DevPath = "/dev/nvme0n1"
	suite.Create(nvme)

	assertCollected := func() {
		ctest.AssertResource(suite, "nvme0n1", func(s *block.SMARTStatus, asrt *assert.Assertions) {
			asrt.Equal("/dev/nvme0n1", s.TypedSpec().DevPath)
		})
	}

	// start from a machine config carrying the document, so that each assertion below observes
	// the status being reaped, instead of racing a controller which didn't reconcile yet.
	cfg := smartMachineConfig(suite)
	suite.Create(cfg)
	assertCollected()

	// a machine config without the document means disabled, and the status is reaped.
	suite.Destroy(cfg)

	cfg = emptyMachineConfig(suite)
	suite.Create(cfg)

	ctest.AssertNoResource[*block.SMARTStatus](suite, "nvme0n1")

	// ... and so does no machine config at all.
	suite.Destroy(cfg)

	cfg = smartMachineConfig(suite)
	suite.Create(cfg)
	assertCollected()

	suite.Destroy(cfg)

	ctest.AssertNoResource[*block.SMARTStatus](suite, "nvme0n1")
}
