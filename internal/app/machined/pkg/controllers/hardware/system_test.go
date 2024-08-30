// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"os"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/go-smbios/smbios"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hardwarectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware"
	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type SystemInfoSuite struct {
	HardwareSuite
}

func (suite *SystemInfoSuite) TestPopulateSystemInformation() {
	stream, err := os.Open("testdata/SuperMicro-Dual-Xeon.dmi")
	suite.Require().NoError(err)

	suite.T().Cleanup(func() { suite.NoError(stream.Close()) })

	version := smbios.Version{Major: 3, Minor: 3, Revision: 0} // dummy version
	s, err := smbios.Decode(stream, version)
	suite.Require().NoError(err)

	suite.Require().NoError(
		suite.runtime.RegisterController(
			&hardwarectrl.SystemInfoController{
				SMBIOS: s,
			},
		),
	)

	suite.startRuntime()

	suite.Require().NoError(suite.state.Create(suite.ctx, runtime.NewMetaLoaded()))

	cpuSpecs := map[string]hardware.ProcessorSpec{
		"CPU-1": {
			Socket:       "CPU 1",
			Manufacturer: "Intel",
			ProductName:  "Intel(R) Xeon(R) CPU E5-2650 v2 @ 2.60GHz",
			MaxSpeed:     4000,
			BootSpeed:    2600,
			Status:       65,
			AssetTag:     "3A65E8E29D76BF8D",
			CoreCount:    8,
			CoreEnabled:  8,
			ThreadCount:  16,
		},
		"CPU-2": {
			Socket:       "CPU 2",
			Manufacturer: "Intel",
			ProductName:  "Intel(R) Xeon(R) CPU E5-2650 v2 @ 2.60GHz",
			MaxSpeed:     4000,
			BootSpeed:    2600,
			Status:       65,
			CoreCount:    8,
			CoreEnabled:  8,
			ThreadCount:  16,
		},
	}

	memorySpecs := map[string]hardware.MemoryModuleSpec{
		"P1-DIMMA1": {
			Size:          4096,
			DeviceLocator: "P1-DIMMA1",
			BankLocator:   "P0_Node0_Channel0_Dimm0",
			Speed:         1333,
			Manufacturer:  "Micron",
			SerialNumber:  "346C4A12",
			AssetTag:      "Dimm0_AssetTag",
			ProductName:   "18KSF51272PZ-1G4K",
		},
		"P1-DIMMA2": {
			Size:          4096,
			DeviceLocator: "P1-DIMMA2",
			BankLocator:   "P0_Node0_Channel0_Dimm1",
			Speed:         1333,
			Manufacturer:  "Kingston",
			SerialNumber:  "D2166C8B",
			AssetTag:      "Dimm1_AssetTag",
			ProductName:   "HP647647-071-HYE",
		},
	}

	for k, v := range cpuSpecs {
		ctest.AssertResource(suite, k, func(r *hardware.Processor, assertions *assert.Assertions) {
			assertions.Equal(v, *r.TypedSpec())
		})
	}

	for k, v := range memorySpecs {
		ctest.AssertResource(suite, k, func(r *hardware.MemoryModule, assertions *assert.Assertions) {
			assertions.Equal(v, *r.TypedSpec())
		})
	}
}

func (suite *SystemInfoSuite) TestUUIDOverwrite() {
	stream, err := os.Open("testdata/SuperMicro-Dual-Xeon.dmi")
	suite.Require().NoError(err)

	suite.T().Cleanup(func() { suite.NoError(stream.Close()) })

	version := smbios.Version{Major: 3, Minor: 3, Revision: 0} // dummy version
	s, err := smbios.Decode(stream, version)
	suite.Require().NoError(err)

	suite.Require().NoError(
		suite.runtime.RegisterController(
			&hardwarectrl.SystemInfoController{
				SMBIOS: s,
			},
		),
	)

	suite.startRuntime()

	suite.Require().NoError(suite.state.Create(suite.ctx, runtime.NewMetaLoaded()))

	key := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.UUIDOverride))
	key.TypedSpec().Value = "00000000-0000-0000-0000-000000000001"

	suite.Require().NoError(suite.state.Create(suite.ctx, key))

	ctest.AssertResource(suite, hardware.SystemInformationID, func(r *hardware.SystemInformation, assertions *assert.Assertions) {
		assertions.Equal("00000000-0000-0000-0000-000000000001", r.TypedSpec().UUID)
	})
}

func (suite *SystemInfoSuite) TestPopulateSystemInformationIsDisabledInContainerMode() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&hardwarectrl.SystemInfoController{
				V1Alpha1Mode: runtimetalos.ModeContainer,
			},
		),
	)

	suite.startRuntime()

	suite.Require().NoError(suite.state.Create(suite.ctx, runtime.NewMetaLoaded()))

	suite.Assert().NoError(retry.Constant(1*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(suite.assertNoResource(*hardware.NewSystemInformation("systeminformation").Metadata())))
}

func TestSystemInfoSyncSuite(t *testing.T) {
	suite.Run(t, new(SystemInfoSuite))
}

func (suite *SystemInfoSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}
