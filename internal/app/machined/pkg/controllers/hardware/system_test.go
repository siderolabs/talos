// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"io"
	"os"
	"testing"
	"time"

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
	ctest.DefaultSuite
}

func (suite *SystemInfoSuite) TestPopulateSystemInformation() {
	stream, err := os.Open("testdata/SuperMicro-Dual-Xeon.dmi")
	suite.Require().NoError(err)

	suite.T().Cleanup(func() { suite.NoError(stream.Close()) })

	version := smbios.Version{Major: 3, Minor: 3, Revision: 0} // dummy version
	s, err := smbios.Decode(stream, version)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.SystemInfoController{
		SMBIOS: s,
	}))

	suite.Create(runtime.NewMetaLoaded())

	systemInformation := hardware.SystemInformationSpec{
		Manufacturer: "Supermicro",
		ProductName:  "SYS-1027R-WRF",
		Version:      "0123456789",
		SerialNumber: "E09626824801435",
		UUID:         "00000000-0000-0000-0000-002590eb9628",
		WakeUpType:   "Power Switch",
		SKUNumber:    "",
		BIOSVersion:  "3.0c",
	}

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
		"P1-DIMMA1-P0_Node0_Channel0_Dimm0": {
			Size:          4096,
			DeviceLocator: "P1-DIMMA1",
			BankLocator:   "P0_Node0_Channel0_Dimm0",
			Speed:         1333,
			Manufacturer:  "Micron",
			SerialNumber:  "346C4A12",
			AssetTag:      "Dimm0_AssetTag",
			ProductName:   "18KSF51272PZ-1G4K",
		},
		"P1-DIMMA2-P0_Node0_Channel0_Dimm1": {
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

	ctest.AssertResource(suite, hardware.SystemInformationID, func(r *hardware.SystemInformation, asrt *assert.Assertions) {
		asrt.Equal(systemInformation, *r.TypedSpec())
	})

	for k, v := range cpuSpecs {
		ctest.AssertResource(suite, k, func(r *hardware.Processor, asrt *assert.Assertions) {
			asrt.Equal(v, *r.TypedSpec())
		})
	}

	for k, v := range memorySpecs {
		ctest.AssertResource(suite, k, func(r *hardware.MemoryModule, asrt *assert.Assertions) {
			asrt.Equal(v, *r.TypedSpec())
		})
	}
}

// TestPopulateMemoryModulesSharedDeviceLocator verifies that memory modules are
// reported individually even when several share the same device locator (some
// boards report multiple `DIMM 0` modules on different banks).
func (suite *SystemInfoSuite) TestPopulateMemoryModulesSharedDeviceLocator() {
	stream, err := os.Open("testdata/MINISFORUM-UM790PRO.dmi")
	suite.Require().NoError(err)

	suite.T().Cleanup(func() { suite.NoError(stream.Close()) })

	version := smbios.Version{Major: 3, Minor: 3, Revision: 0} // dummy version
	s, err := smbios.Decode(stream, version)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.SystemInfoController{
		SMBIOS: s,
	}))

	suite.Create(runtime.NewMetaLoaded())

	memorySpecs := map[string]hardware.MemoryModuleSpec{
		"DIMM-0-P0-CHANNEL-A": {
			Size:          32768,
			DeviceLocator: "DIMM 0",
			BankLocator:   "P0 CHANNEL A",
			Speed:         5600,
			Manufacturer:  "Micron Technology",
			SerialNumber:  "EB159FFA",
			ProductName:   "CT32G56C46S5.M16B2",
		},
		"DIMM-0-P0-CHANNEL-B": {
			Size:          32768,
			DeviceLocator: "DIMM 0",
			BankLocator:   "P0 CHANNEL B",
			Speed:         5600,
			Manufacturer:  "Micron Technology",
			SerialNumber:  "EB159FD4",
			ProductName:   "CT32G56C46S5.M16B2",
		},
	}

	for k, v := range memorySpecs {
		ctest.AssertResource(suite, k, func(r *hardware.MemoryModule, asrt *assert.Assertions) {
			asrt.Equal(v, *r.TypedSpec())
		})
	}
}

func (suite *SystemInfoSuite) TestPopulateSystemInformationEmpty() {
	version := smbios.Version{Major: 3, Minor: 3, Revision: 0} // dummy version
	s, err := smbios.Decode(io.NewSectionReader(nil, 0, 0), version)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.SystemInfoController{
		SMBIOS: s,
	}))

	suite.Create(runtime.NewMetaLoaded())

	ctest.AssertResource(suite, "UNKNOWN", func(r *hardware.MemoryModule, asrt *assert.Assertions) {
		asrt.Equal("UNKNOWN", r.TypedSpec().Manufacturer)
		asrt.NotZero(r.TypedSpec().Size)
	})
}

func (suite *SystemInfoSuite) TestUUIDOverwrite() {
	stream, err := os.Open("testdata/SuperMicro-Dual-Xeon.dmi")
	suite.Require().NoError(err)

	suite.T().Cleanup(func() { suite.NoError(stream.Close()) })

	version := smbios.Version{Major: 3, Minor: 3, Revision: 0} // dummy version
	s, err := smbios.Decode(stream, version)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.SystemInfoController{
		SMBIOS: s,
	}))

	suite.Create(runtime.NewMetaLoaded())

	key := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.UUIDOverride))
	key.TypedSpec().Value = "00000000-0000-0000-0000-000000000001"

	suite.Create(key)

	ctest.AssertResource(suite, hardware.SystemInformationID, func(r *hardware.SystemInformation, asrt *assert.Assertions) {
		asrt.Equal("00000000-0000-0000-0000-000000000001", r.TypedSpec().UUID)
	})
}

func (suite *SystemInfoSuite) TestPopulateSystemInformationIsDisabledInContainerMode() {
	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.SystemInfoController{
		V1Alpha1Mode: runtimetalos.ModeContainer,
	}))

	suite.Create(runtime.NewMetaLoaded())

	ctest.AssertNoResource[*hardware.SystemInformation](suite, hardware.SystemInformationID)
}

func TestSystemInfoSyncSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &SystemInfoSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
		},
	})
}
