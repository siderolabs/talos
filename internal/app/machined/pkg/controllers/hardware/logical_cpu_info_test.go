// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hardwarectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

type LogicalCPUInfoSuite struct {
	ctest.DefaultSuite
}

// fakeProcCPUInfo writes a minimal /proc/cpuinfo into procfsRoot/cpuinfo.
//
// Two logical CPUs sharing socket 0; the second one carries no bugs to verify
// the field handles absence cleanly.
func fakeProcCPUInfo(t *testing.T, procfsRoot string) {
	t.Helper()

	content := `processor	: 0
vendor_id	: AuthenticAMD
cpu family	: 25
model		: 116
model name	: AMD Test CPU
microcode	: 0xb600032
bugs		: fake_bug_a fake_bug_b
flags		: fpu

processor	: 1
vendor_id	: AuthenticAMD
cpu family	: 25
model		: 116
model name	: AMD Test CPU
microcode	: 0xb600032
flags		: fpu

`

	require := assert.New(t)
	require.NoError(os.WriteFile(filepath.Join(procfsRoot, "cpuinfo"), []byte(content), 0o644))
}

// fakeSysfsCPU lays out /sys/devices/system/cpu/<id>/topology/{physical_package_id,core_id}
// and a node<NUMA> entry for one logical CPU. On real systems the entry is a
// symlink to the NUMA node directory, but readNUMANode only inspects entry
// names so we just touch a plain file here.
func fakeSysfsCPU(t *testing.T, sysfsRoot, cpuID string, socket, core, numa int) {
	t.Helper()

	require := assert.New(t)

	topology := filepath.Join(sysfsRoot, cpuID, "topology")
	require.NoError(os.MkdirAll(topology, 0o755))
	require.NoError(os.WriteFile(filepath.Join(topology, "physical_package_id"), []byte(itoa(socket)+"\n"), 0o644))
	require.NoError(os.WriteFile(filepath.Join(topology, "core_id"), []byte(itoa(core)+"\n"), 0o644))

	// Touch a placeholder file inside an empty "node<N>" entry; readNUMANode
	// only inspects directory names so an empty file is enough for the test.
	require.NoError(os.WriteFile(filepath.Join(sysfsRoot, cpuID, "node"+itoa(numa)), nil, 0o644))
}

func itoa(n int) string {
	// strconv-free helper so the test file stays import-light.
	if n == 0 {
		return "0"
	}

	neg := n < 0
	if neg {
		n = -n
	}

	var b [20]byte

	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}

	if neg {
		i--
		b[i] = '-'
	}

	return string(b[i:])
}

func (suite *LogicalCPUInfoSuite) TestReconcilesFromFixtures() {
	procfsRoot := suite.T().TempDir()
	sysfsRoot := suite.T().TempDir()

	fakeProcCPUInfo(suite.T(), procfsRoot)
	// cpu0 and cpu1 share socket 0 / NUMA 0; SMT siblings (same core_id).
	fakeSysfsCPU(suite.T(), sysfsRoot, "cpu0", 0, 0, 0)
	fakeSysfsCPU(suite.T(), sysfsRoot, "cpu1", 0, 0, 0)

	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.LogicalCPUInfoController{
		ProcfsPath:   procfsRoot,
		SysfsCPUPath: sysfsRoot,
	}))

	ctest.AssertResource(suite, "cpu0", func(r *hardware.LogicalCPUInfo, asrt *assert.Assertions) {
		spec := r.TypedSpec()
		asrt.Equal("0xb600032", spec.Microcode)
		asrt.Equal(uint32(0), spec.Socket)
		asrt.Equal(uint32(0), spec.Core)
		asrt.Equal(uint32(0), spec.NumaNode)
		asrt.Equal([]string{"fake_bug_a", "fake_bug_b"}, spec.Bugs)
	})

	ctest.AssertResource(suite, "cpu1", func(r *hardware.LogicalCPUInfo, asrt *assert.Assertions) {
		spec := r.TypedSpec()
		asrt.Equal("0xb600032", spec.Microcode)
		asrt.Equal(uint32(0), spec.Socket)
		asrt.Equal(uint32(0), spec.Core)
		asrt.Equal(uint32(0), spec.NumaNode)
		asrt.Empty(spec.Bugs)
	})
}

func (suite *LogicalCPUInfoSuite) TestTopologyAbsent() {
	// procfs present, sysfs entirely missing — controller must still publish
	// resources with topology fields defaulted to 0 rather than erroring out.
	procfsRoot := suite.T().TempDir()
	fakeProcCPUInfo(suite.T(), procfsRoot)

	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.LogicalCPUInfoController{
		ProcfsPath:   procfsRoot,
		SysfsCPUPath: suite.T().TempDir(),
	}))

	ctest.AssertResource(suite, "cpu0", func(r *hardware.LogicalCPUInfo, asrt *assert.Assertions) {
		spec := r.TypedSpec()
		asrt.Equal("0xb600032", spec.Microcode)
		asrt.Equal(uint32(0), spec.Socket)
		asrt.Equal(uint32(0), spec.Core)
		asrt.Equal(uint32(0), spec.NumaNode)
	})
}

func TestLogicalCPUInfoSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &LogicalCPUInfoSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
		},
	})
}
