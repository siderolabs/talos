// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"testing"
	"time"

	"github.com/prometheus/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hardwarectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

type CPUInfoSuite struct {
	ctest.DefaultSuite
}

func (suite *CPUInfoSuite) TestPopulateCPUCores() {
	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.CPUInfoController{
		ProcfsPath: "testdata/x86",
	}))

	expected := map[string]hardware.CPUCoreSpec{
		"0-0": {
			Socket:           "0",
			CoreID:           "0",
			LogicalCPUs:      []uint32{0, 2},
			VendorID:         "GenuineIntel",
			CPUFamily:        "6",
			Model:            "158",
			ModelName:        "Intel(R) Core(TM) i7-8700 CPU @ 3.20GHz",
			Stepping:         "10",
			Microcode:        "0xf4",
			CacheSize:        "12288 KB",
			CoresPerSocket:   2,
			ThreadsPerSocket: 4,
			Flags:            []string{"fpu", "vme", "de", "pse", "tsc", "msr"},
			Bugs:             []string{"cpu_meltdown", "l1tf", "mds"},
			BogoMips:         6384.00,
			AddressSizes:     "39 bits physical, 48 bits virtual",
		},
		"0-1": {
			Socket:           "0",
			CoreID:           "1",
			LogicalCPUs:      []uint32{1, 3},
			VendorID:         "GenuineIntel",
			CPUFamily:        "6",
			Model:            "158",
			ModelName:        "Intel(R) Core(TM) i7-8700 CPU @ 3.20GHz",
			Stepping:         "10",
			Microcode:        "0xf4",
			CacheSize:        "12288 KB",
			CoresPerSocket:   2,
			ThreadsPerSocket: 4,
			Flags:            []string{"fpu", "vme", "de", "pse", "tsc", "msr"},
			Bugs:             []string{"cpu_meltdown", "l1tf", "mds"},
			BogoMips:         6384.00,
			AddressSizes:     "39 bits physical, 48 bits virtual",
		},
	}

	for id, spec := range expected {
		ctest.AssertResource(suite, id, func(r *hardware.CPUCore, asrt *assert.Assertions) {
			asrt.Equal(spec, *r.TypedSpec())
		})
	}
}

func TestCPUInfoSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CPUInfoSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
		},
	})
}

func TestGroupCPUInfo(t *testing.T) {
	t.Parallel()

	t.Run("grouped by core", func(t *testing.T) {
		t.Parallel()

		fs, err := procfs.NewFS("testdata/x86")
		require.NoError(t, err)

		cpus, err := fs.CPUInfo()
		require.NoError(t, err)

		cores := hardwarectrl.GroupCPUInfo(cpus)

		require.Len(t, cores, 2)
		assert.Equal(t, []uint32{0, 2}, cores["0-0"].LogicalCPUs)
		assert.Equal(t, []uint32{1, 3}, cores["0-1"].LogicalCPUs)
	})

	t.Run("fallback to processor number", func(t *testing.T) {
		t.Parallel()

		// On systems without physical/core ids (e.g. some ARM), each logical CPU is its own core.
		fs, err := procfs.NewFS("testdata/arm")
		require.NoError(t, err)

		cpus, err := fs.CPUInfo()
		require.NoError(t, err)

		cores := hardwarectrl.GroupCPUInfo(cpus)

		require.Len(t, cores, 2)
		assert.Equal(t, []uint32{0}, cores["0"].LogicalCPUs)
		assert.Equal(t, []uint32{1}, cores["1"].LogicalCPUs)
	})
}
