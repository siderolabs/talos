// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"
	"github.com/talos-systems/go-smbios/smbios"

	hardwarectrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/hardware"
	"github.com/talos-systems/talos/pkg/machinery/resources/hardware"
)

type SystemInfoSuite struct {
	HardwareSuite
}

func (suite *SystemInfoSuite) TestPopulateSystemInformation() {
	stream, err := os.Open("testdata/SuperMicro-Dual-Xeon.dmi")
	suite.Require().NoError(err)

	//nolint: errcheck
	defer stream.Close()

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

	memorySpecs := map[string]hardware.MemorySpec{
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
		suite.Assert().NoError(
			retry.Constant(1*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
				suite.assertResource(*hardware.NewProcessorInfo(k).Metadata(), func(r resource.Resource) error {
					status := *r.(*hardware.Processor).TypedSpec()
					if !suite.Assert().Equal(v, status) {
						return retry.ExpectedError(fmt.Errorf("cpu status doesn't match: %v != %v", v, status))
					}

					return nil
				}),
			),
		)
	}

	for k, v := range memorySpecs {
		suite.Assert().NoError(
			retry.Constant(1*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
				suite.assertResource(*hardware.NewMemoryInfo(k).Metadata(), func(r resource.Resource) error {
					status := *r.(*hardware.Memory).TypedSpec()
					if !suite.Assert().Equal(v, status) {
						return retry.ExpectedError(fmt.Errorf("memory status doesn't match: %v != %v", v, status))
					}

					return nil
				}),
			),
		)
	}
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
