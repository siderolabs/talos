// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"context"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hardwarectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// writeSysfs materializes a fake sysfs tree, newline-terminating values as the kernel does.
//
// Values are otherwise exactly what the kernel prints, trailing space included: cpufreq prints CPU
// masks as `0 1 2 3`, governor and EPP lists with a space after each item, and the perf PMUs print
// CPU masks in the `0-3,8` list format.
func writeSysfs(t *testing.T, root string, files map[string]string) {
	t.Helper()

	for path, contents := range files {
		full := filepath.Join(root, path)

		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(contents+"\n"), 0o644))
	}
}

// fakeSysfs reads a fake sysfs tree, failing every read under an inactive cpufreq policy with
// EBUSY, as the kernel does once all CPUs of a policy are offline.
type fakeSysfs struct {
	mu        sync.Mutex
	inactive  map[string]bool
	busyReads map[string]int
}

func (f *fakeSysfs) setInactive(policy string, inactive bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.inactive == nil {
		f.inactive = map[string]bool{}
	}

	f.inactive[policy] = inactive
}

// busyReadCount returns how many reads of a policy attribute failed with EBUSY.
func (f *fakeSysfs) busyReadCount(policy, name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.busyReads[policy+"/"+name]
}

func (f *fakeSysfs) readFile(name string) ([]byte, error) {
	f.mu.Lock()

	for policy, inactive := range f.inactive {
		if inactive && strings.Contains(filepath.ToSlash(name), "/cpufreq/"+policy+"/") {
			if f.busyReads == nil {
				f.busyReads = map[string]int{}
			}

			f.busyReads[policy+"/"+filepath.Base(name)]++
			f.mu.Unlock()

			return nil, &fs.PathError{Op: "read", Path: name, Err: syscall.EBUSY}
		}
	}

	f.mu.Unlock()

	return os.ReadFile(name)
}

func readPolicyAttr(t *testing.T, root, policy, name string) string {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(root, "devices/system/cpu/cpufreq", policy, name))
	require.NoError(t, err)

	return strings.TrimSpace(string(contents))
}

// cpufreqTree covers the shapes an x86 cpufreq driver can present, rather than one real machine:
//
//   - policy0 mirrors a real intel_cpufreq box: one CPU per policy, no EPP, no base frequency;
//   - policy1 drives several CPUs at once, as acpi-cpufreq does under _PSD hardware coordination;
//   - policy2 is HWP-style, with EPP and a base frequency;
//   - policy3 is inactive (see CPUScalingSuite), so the kernel refuses to read any of it;
//   - policy5 is a low power E-core of a three-tier hybrid CPU.
func cpufreqTree() map[string]string {
	return map[string]string{
		"devices/system/cpu/cpufreq/policy0/scaling_driver":              "intel_cpufreq",
		"devices/system/cpu/cpufreq/policy0/affected_cpus":               "0",
		"devices/system/cpu/cpufreq/policy0/related_cpus":                "0",
		"devices/system/cpu/cpufreq/policy0/scaling_available_governors": "conservative ondemand userspace powersave performance schedutil ",
		"devices/system/cpu/cpufreq/policy0/scaling_governor":            "schedutil",
		"devices/system/cpu/cpufreq/policy0/scaling_min_freq":            "1200000",
		"devices/system/cpu/cpufreq/policy0/scaling_max_freq":            "3300000",
		"devices/system/cpu/cpufreq/policy0/cpuinfo_min_freq":            "1200000",
		"devices/system/cpu/cpufreq/policy0/cpuinfo_max_freq":            "3300000",
		"devices/system/cpu/cpufreq/policy0/scaling_cur_freq":            "2712000",

		"devices/system/cpu/cpufreq/policy1/scaling_driver":              "acpi-cpufreq",
		"devices/system/cpu/cpufreq/policy1/affected_cpus":               "1 2 3",
		"devices/system/cpu/cpufreq/policy1/related_cpus":                "1 2 3",
		"devices/system/cpu/cpufreq/policy1/scaling_available_governors": "ondemand performance ",
		"devices/system/cpu/cpufreq/policy1/scaling_governor":            "performance",
		"devices/system/cpu/cpufreq/policy1/scaling_min_freq":            "800000",
		"devices/system/cpu/cpufreq/policy1/scaling_max_freq":            "2400000",
		"devices/system/cpu/cpufreq/policy1/cpuinfo_min_freq":            "800000",
		"devices/system/cpu/cpufreq/policy1/cpuinfo_max_freq":            "2400000",

		"devices/system/cpu/cpufreq/policy2/scaling_driver":                           "intel_pstate",
		"devices/system/cpu/cpufreq/policy2/affected_cpus":                            "4",
		"devices/system/cpu/cpufreq/policy2/related_cpus":                             "4",
		"devices/system/cpu/cpufreq/policy2/scaling_available_governors":              "performance powersave",
		"devices/system/cpu/cpufreq/policy2/scaling_governor":                         "powersave",
		"devices/system/cpu/cpufreq/policy2/energy_performance_preference":            "balance_performance",
		"devices/system/cpu/cpufreq/policy2/energy_performance_available_preferences": "default performance balance_performance balance_power power ",
		"devices/system/cpu/cpufreq/policy2/scaling_min_freq":                         "400000",
		"devices/system/cpu/cpufreq/policy2/scaling_max_freq":                         "3900000",
		"devices/system/cpu/cpufreq/policy2/cpuinfo_min_freq":                         "400000",
		"devices/system/cpu/cpufreq/policy2/cpuinfo_max_freq":                         "3900000",
		"devices/system/cpu/cpufreq/policy2/base_frequency":                           "2100000",

		"devices/system/cpu/cpufreq/policy3/scaling_driver":              "acpi-cpufreq",
		"devices/system/cpu/cpufreq/policy3/affected_cpus":               "8",
		"devices/system/cpu/cpufreq/policy3/related_cpus":                "8",
		"devices/system/cpu/cpufreq/policy3/scaling_available_governors": "performance ",
		"devices/system/cpu/cpufreq/policy3/scaling_governor":            "performance",

		"devices/system/cpu/cpufreq/policy5/scaling_driver":              "intel_pstate",
		"devices/system/cpu/cpufreq/policy5/affected_cpus":               "5",
		"devices/system/cpu/cpufreq/policy5/related_cpus":                "5",
		"devices/system/cpu/cpufreq/policy5/scaling_available_governors": "performance powersave",
		"devices/system/cpu/cpufreq/policy5/scaling_governor":            "powersave",

		"devices/cpu_core/cpus":     "0-2",
		"devices/cpu_atom/cpus":     "3-4,8",
		"devices/cpu_lowpower/cpus": "5",

		"devices/system/cpu/cpu0/cpu_capacity": "1024",
		"devices/system/cpu/cpu4/cpu_capacity": "512",
		"devices/system/cpu/cpu5/cpu_capacity": "384",
	}
}

type CPUScalingSuite struct {
	ctest.DefaultSuite

	root        string
	reconcileCh chan struct{}
	sysfs       *fakeSysfs
}

func (suite *CPUScalingSuite) SetupTest() {
	suite.DefaultSuite.SetupTest()

	suite.root = suite.T().TempDir()
	suite.reconcileCh = make(chan struct{}, 1)
	suite.sysfs = &fakeSysfs{}

	writeSysfs(suite.T(), suite.root, cpufreqTree())
	suite.sysfs.setInactive("policy3", true)

	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.CPUScalingController{
		SysfsPath:   suite.root,
		ReconcileCh: suite.reconcileCh,
		ReadFile:    suite.sysfs.readFile,
	}))
}

func (suite *CPUScalingSuite) TestReportPolicies() {
	expected := map[string]hardware.CPUScalingStatusSpec{
		"policy0": {
			Driver:                 "intel_cpufreq",
			AffectedCPUs:           []uint32{0},
			RelatedCPUs:            []uint32{0},
			AvailableGovernors:     []string{"conservative", "ondemand", "userspace", "powersave", "performance", "schedutil"},
			Governor:               "schedutil",
			CPUInfoMinFrequencyKhz: 1200000,
			CPUInfoMaxFrequencyKhz: 3300000,
			ScalingMinFrequencyKhz: 1200000,
			ScalingMaxFrequencyKhz: 3300000,
			CPUCapacity:            1024,
			CoreType:               hardware.CoreTypePerformance,
		},
		"policy1": {
			Driver:                 "acpi-cpufreq",
			AffectedCPUs:           []uint32{1, 2, 3},
			RelatedCPUs:            []uint32{1, 2, 3},
			AvailableGovernors:     []string{"ondemand", "performance"},
			Governor:               "performance",
			CPUInfoMinFrequencyKhz: 800000,
			CPUInfoMaxFrequencyKhz: 2400000,
			ScalingMinFrequencyKhz: 800000,
			ScalingMaxFrequencyKhz: 2400000,
			CoreType:               hardware.CoreTypePerformance,
		},
		"policy2": {
			Driver:                      "intel_pstate",
			AffectedCPUs:                []uint32{4},
			RelatedCPUs:                 []uint32{4},
			AvailableGovernors:          []string{"performance", "powersave"},
			AvailableEPPs:               []string{"default", "performance", "balance_performance", "balance_power", "power"},
			Governor:                    "powersave",
			EnergyPerformancePreference: "balance_performance",
			CPUInfoMinFrequencyKhz:      400000,
			CPUInfoMaxFrequencyKhz:      3900000,
			ScalingMinFrequencyKhz:      400000,
			ScalingMaxFrequencyKhz:      3900000,
			BaseFrequencyKhz:            2100000,
			CPUCapacity:                 512,
			CoreType:                    hardware.CoreTypeEfficiency,
		},
		"policy5": {
			Driver:             "intel_pstate",
			AffectedCPUs:       []uint32{5},
			RelatedCPUs:        []uint32{5},
			AvailableGovernors: []string{"performance", "powersave"},
			Governor:           "powersave",
			CPUCapacity:        384,
			CoreType:           hardware.CoreTypeEfficiency,
		},
	}

	assertStatuses(suite, expected)
}

// assertStatuses asserts the controller reports exactly the expected policies.
func assertStatuses(suite interface {
	ctest.Suiter
	Ctx() context.Context
}, expected map[string]hardware.CPUScalingStatusSpec,
) {
	for id, spec := range expected {
		ctest.AssertResource(suite, id, func(r *hardware.CPUScalingStatus, asrt *assert.Assertions) {
			asrt.Equal(spec, *r.TypedSpec())
		})
	}

	ctx, cancel := context.WithTimeout(suite.Ctx(), 10*time.Second)
	defer cancel()

	rtestutils.AssertLength[*hardware.CPUScalingStatus](ctx, suite.T(), suite.State(), len(expected))
}

// TestMalformedPolicySkipped covers a policy presenting something unexpected: it is left out, and
// the rest are still reported. The value is in the list format, which cpufreq never uses.
func (suite *CPUScalingSuite) TestMalformedPolicySkipped() {
	ctest.AssertResource(suite, "policy1", func(r *hardware.CPUScalingStatus, asrt *assert.Assertions) {
		asrt.Equal([]uint32{1, 2, 3}, r.TypedSpec().AffectedCPUs)
	})

	writeSysfs(suite.T(), suite.root, map[string]string{
		"devices/system/cpu/cpufreq/policy1/affected_cpus": "1-3",
	})

	suite.reconcileCh <- struct{}{}

	ctest.AssertNoResource[*hardware.CPUScalingStatus](suite, "policy1")
	ctest.AssertResource(suite, "policy0", func(r *hardware.CPUScalingStatus, asrt *assert.Assertions) {
		asrt.Equal("intel_cpufreq", r.TypedSpec().Driver)
	})
}

// TestInactivePolicy covers all CPUs of a policy going offline and coming back: the kernel refuses
// any access meanwhile, so the policy is not reported, and restoring its settings waits.
func (suite *CPUScalingSuite) TestInactivePolicy() {
	spec := hardware.NewCPUScalingSpec("policy0")
	spec.TypedSpec().Governor = "powersave"

	suite.Create(spec)

	ctest.AssertResource(suite, "policy0", func(r *hardware.CPUScalingStatus, asrt *assert.Assertions) {
		asrt.Equal("powersave", r.TypedSpec().Governor)
	})

	suite.sysfs.setInactive("policy0", true)

	suite.reconcileCh <- struct{}{}

	ctest.AssertNoResource[*hardware.CPUScalingStatus](suite, "policy0")

	// the other policies are still reported meanwhile
	writeSysfs(suite.T(), suite.root, map[string]string{
		"devices/system/cpu/cpufreq/policy1/scaling_governor": "ondemand",
	})

	suite.reconcileCh <- struct{}{}

	ctest.AssertResource(suite, "policy1", func(r *hardware.CPUScalingStatus, asrt *assert.Assertions) {
		asrt.Equal("ondemand", r.TypedSpec().Governor)
	})

	suite.Destroy(spec)

	// restoring reads the attribute first, which fails while the policy is inactive
	suite.AssertWithin(3*time.Second, 10*time.Millisecond, func() error {
		if suite.sysfs.busyReadCount("policy0", "scaling_governor") == 0 {
			return retry.ExpectedErrorf("no attempt to restore yet")
		}

		return nil
	})

	assert.Equal(suite.T(), "powersave", readPolicyAttr(suite.T(), suite.root, "policy0", "scaling_governor"))

	suite.sysfs.setInactive("policy0", false)

	suite.reconcileCh <- struct{}{}

	suite.AssertWithin(3*time.Second, 10*time.Millisecond, func() error {
		if got := readPolicyAttr(suite.T(), suite.root, "policy0", "scaling_governor"); got != "schedutil" {
			return retry.ExpectedErrorf("scaling_governor is %q", got)
		}

		return nil
	})

	ctest.AssertResource(suite, "policy0", func(r *hardware.CPUScalingStatus, asrt *assert.Assertions) {
		asrt.Equal("schedutil", r.TypedSpec().Governor)
	})
}

// TestApplyReportedImmediately covers the reason applying and reporting share a controller: the
// status must reflect the write that just happened, with no polling to catch up.
func (suite *CPUScalingSuite) TestApplyReportedImmediately() {
	spec := hardware.NewCPUScalingSpec("policy0")
	spec.TypedSpec().Governor = "powersave"
	spec.TypedSpec().MinFrequencyKhz = 1400000
	spec.TypedSpec().MaxFrequencyKhz = 2800000

	suite.Create(spec)

	ctest.AssertResource(suite, "policy0", func(r *hardware.CPUScalingStatus, asrt *assert.Assertions) {
		asrt.Equal("powersave", r.TypedSpec().Governor)
		asrt.EqualValues(1400000, r.TypedSpec().ScalingMinFrequencyKhz)
		asrt.EqualValues(2800000, r.TypedSpec().ScalingMaxFrequencyKhz)
	})
}

func (suite *CPUScalingSuite) TestRestoreOnRemoval() {
	spec := hardware.NewCPUScalingSpec("policy0")
	spec.TypedSpec().Governor = "powersave"
	spec.TypedSpec().MaxFrequencyKhz = 2800000

	suite.Create(spec)

	suite.AssertWithin(3*time.Second, 10*time.Millisecond, func() error {
		if got := readPolicyAttr(suite.T(), suite.root, "policy0", "scaling_governor"); got != "powersave" {
			return retry.ExpectedErrorf("scaling_governor is %q", got)
		}

		return nil
	})

	suite.Destroy(spec)

	suite.AssertWithin(3*time.Second, 10*time.Millisecond, func() error {
		if got := readPolicyAttr(suite.T(), suite.root, "policy0", "scaling_governor"); got != "schedutil" {
			return retry.ExpectedErrorf("scaling_governor is %q", got)
		}

		if got := readPolicyAttr(suite.T(), suite.root, "policy0", "scaling_max_freq"); got != "3300000" {
			return retry.ExpectedErrorf("scaling_max_freq is %q", got)
		}

		return nil
	})
}

// TestDroppedFieldRestored covers removing one field from a config and leaving the rest.
func (suite *CPUScalingSuite) TestDroppedFieldRestored() {
	spec := hardware.NewCPUScalingSpec("policy0")
	spec.TypedSpec().Governor = "powersave"
	spec.TypedSpec().MaxFrequencyKhz = 2800000

	suite.Create(spec)

	suite.AssertWithin(3*time.Second, 10*time.Millisecond, func() error {
		if got := readPolicyAttr(suite.T(), suite.root, "policy0", "scaling_max_freq"); got != "2800000" {
			return retry.ExpectedErrorf("scaling_max_freq is %q", got)
		}

		return nil
	})

	ctest.UpdateWithConflicts(suite, spec, func(r *hardware.CPUScalingSpec) error {
		r.TypedSpec().MaxFrequencyKhz = 0

		return nil
	})

	suite.AssertWithin(3*time.Second, 10*time.Millisecond, func() error {
		if got := readPolicyAttr(suite.T(), suite.root, "policy0", "scaling_max_freq"); got != "3300000" {
			return retry.ExpectedErrorf("scaling_max_freq is %q", got)
		}

		return nil
	})

	assert.Equal(suite.T(), "powersave", readPolicyAttr(suite.T(), suite.root, "policy0", "scaling_governor"))
}

// TestUnsupportedValueSkipped covers a governor the driver does not offer.
func (suite *CPUScalingSuite) TestUnsupportedValueSkipped() {
	spec := hardware.NewCPUScalingSpec("policy1")
	spec.TypedSpec().Governor = "schedutil"
	spec.TypedSpec().MaxFrequencyKhz = 2000000

	suite.Create(spec)

	suite.AssertWithin(3*time.Second, 10*time.Millisecond, func() error {
		if got := readPolicyAttr(suite.T(), suite.root, "policy1", "scaling_max_freq"); got != "2000000" {
			return retry.ExpectedErrorf("scaling_max_freq is %q", got)
		}

		return nil
	})

	assert.Equal(suite.T(), "performance", readPolicyAttr(suite.T(), suite.root, "policy1", "scaling_governor"))
}

// TestEPPSkippedWhenAbsent covers a driver which does not implement EPP at all.
func (suite *CPUScalingSuite) TestEPPSkippedWhenAbsent() {
	spec := hardware.NewCPUScalingSpec("policy0")
	spec.TypedSpec().EnergyPerformancePreference = "power"
	spec.TypedSpec().Governor = "powersave"

	suite.Create(spec)

	suite.AssertWithin(3*time.Second, 10*time.Millisecond, func() error {
		if got := readPolicyAttr(suite.T(), suite.root, "policy0", "scaling_governor"); got != "powersave" {
			return retry.ExpectedErrorf("scaling_governor is %q", got)
		}

		return nil
	})

	_, err := os.Stat(filepath.Join(suite.root, "devices/system/cpu/cpufreq/policy0/energy_performance_preference"))
	assert.ErrorIs(suite.T(), err, os.ErrNotExist)
}

func (suite *CPUScalingSuite) TestPolicyRemoved() {
	ctest.AssertResource(suite, "policy1", func(r *hardware.CPUScalingStatus, asrt *assert.Assertions) {
		asrt.Equal("acpi-cpufreq", r.TypedSpec().Driver)
	})

	suite.Require().NoError(os.RemoveAll(filepath.Join(suite.root, "devices/system/cpu/cpufreq/policy1")))

	suite.reconcileCh <- struct{}{}

	ctest.AssertNoResource[*hardware.CPUScalingStatus](suite, "policy1")
	ctest.AssertResource(suite, "policy0", func(r *hardware.CPUScalingStatus, asrt *assert.Assertions) {
		asrt.Equal("intel_cpufreq", r.TypedSpec().Driver)
	})
}

func TestCPUScalingSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CPUScalingSuite{
		Timeout: 10 * time.Second,
	})
}

type CPUScalingNoDriverSuite struct {
	ctest.DefaultSuite
}

// TestNoCPUFreq covers a machine with no cpufreq driver at all (most VMs). A policy added afterwards
// is picked up, which is what proves the absence came from an empty sysfs and not a dead controller.
func (suite *CPUScalingNoDriverSuite) TestNoCPUFreq() {
	root := suite.T().TempDir()
	reconcileCh := make(chan struct{}, 1)

	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.CPUScalingController{
		SysfsPath:   root,
		ReconcileCh: reconcileCh,
	}))

	ctest.AssertNoResource[*hardware.CPUScalingStatus](suite, "policy0")

	writeSysfs(suite.T(), root, cpufreqTree())

	reconcileCh <- struct{}{}

	ctest.AssertResource(suite, "policy0", func(r *hardware.CPUScalingStatus, asrt *assert.Assertions) {
		asrt.Equal("intel_cpufreq", r.TypedSpec().Driver)
	})
}

func TestCPUScalingNoDriverSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CPUScalingNoDriverSuite{
		Timeout: 10 * time.Second,
	})
}

// armTrees holds the cpufreq trees of Arm systems. Each is modeled on the board named, with the
// values a mainline kernel prints for it, but is not a capture from one.
var armTrees = map[string]struct {
	tree     map[string]string
	expected map[string]hardware.CPUScalingStatusSpec
}{
	// RK3566 (e.g. SOQuartz CM4): four Cortex-A55 on one clock, so one policy drives them all
	"rk3566": {
		tree: mergeTrees(
			armPolicy("policy0", "0 1 2 3", 408000, 1800000),
			armCPUs(midrCortexA55, 1024, 0, 1, 2, 3),
		),
		expected: map[string]hardware.CPUScalingStatusSpec{
			"policy0": armStatus([]uint32{0, 1, 2, 3}, []uint32{0, 1, 2, 3}, 408000, 1800000, 1024, ""),
		},
	},
	// RK3588: four Cortex-A55 and two clusters of two Cortex-A76, with CPU 3 taken offline
	"rk3588": {
		tree: mergeTrees(
			armPolicy("policy0", "0 1 2", 408000, 1800000),
			map[string]string{"devices/system/cpu/cpufreq/policy0/related_cpus": "0 1 2 3"},
			armPolicy("policy4", "4 5", 408000, 2304000),
			armPolicy("policy6", "6 7", 408000, 2304000),
			armCPUs(midrCortexA55, 414, 0, 1, 2),
			armCPUs(midrCortexA76, 1024, 4, 5, 6, 7),
		),
		expected: map[string]hardware.CPUScalingStatusSpec{
			"policy0": armStatus([]uint32{0, 1, 2}, []uint32{0, 1, 2, 3}, 408000, 1800000, 414, hardware.CoreTypeEfficiency),
			"policy4": armStatus([]uint32{4, 5}, []uint32{4, 5}, 408000, 2304000, 1024, hardware.CoreTypePerformance),
			"policy6": armStatus([]uint32{6, 7}, []uint32{6, 7}, 408000, 2304000, 1024, hardware.CoreTypePerformance),
		},
	},
	// eight Cortex-A53 in two clusters clocked differently: the capacities differ, the cores do not
	"octa-a53": {
		tree: mergeTrees(
			armPolicy("policy0", "0 1 2 3", 600000, 1800000),
			armPolicy("policy4", "4 5 6 7", 600000, 1400000),
			armCPUs(midrCortexA53, 1024, 0, 1, 2, 3),
			armCPUs(midrCortexA53r1, 796, 4, 5, 6, 7),
		),
		expected: map[string]hardware.CPUScalingStatusSpec{
			"policy0": armStatus([]uint32{0, 1, 2, 3}, []uint32{0, 1, 2, 3}, 600000, 1800000, 1024, ""),
			"policy4": armStatus([]uint32{4, 5, 6, 7}, []uint32{4, 5, 6, 7}, 600000, 1400000, 796, ""),
		},
	},
	// a big.LITTLE system whose firmware does not rate the cores, so all report the default capacity
	"unrated": {
		tree: mergeTrees(
			armPolicy("policy0", "0 1 2 3", 408000, 1800000),
			armPolicy("policy4", "4 5", 408000, 2304000),
			armCPUs(midrCortexA55, 1024, 0, 1, 2, 3),
			armCPUs(midrCortexA76, 1024, 4, 5),
		),
		expected: map[string]hardware.CPUScalingStatusSpec{
			"policy0": armStatus([]uint32{0, 1, 2, 3}, []uint32{0, 1, 2, 3}, 408000, 1800000, 1024, ""),
			"policy4": armStatus([]uint32{4, 5}, []uint32{4, 5}, 408000, 2304000, 1024, ""),
		},
	},
}

// MIDR_EL1 values as the kernel prints them; the variant and revision differ between the two A53s.
const (
	midrCortexA53   = "0x00000000410fd034"
	midrCortexA53r1 = "0x00000000411fd030"
	midrCortexA55   = "0x00000000412fd050"
	midrCortexA76   = "0x00000000414fd0b0"
)

const armGovernors = "conservative ondemand userspace powersave performance schedutil "

func armPolicy(id, cpus string, minKhz, maxKhz int) map[string]string {
	dir := "devices/system/cpu/cpufreq/" + id + "/"

	return map[string]string{
		dir + "scaling_driver":              "cpufreq-dt",
		dir + "affected_cpus":               cpus,
		dir + "related_cpus":                cpus,
		dir + "scaling_available_governors": armGovernors,
		dir + "scaling_governor":            "schedutil",
		dir + "cpuinfo_min_freq":            strconv.Itoa(minKhz),
		dir + "cpuinfo_max_freq":            strconv.Itoa(maxKhz),
		dir + "scaling_min_freq":            strconv.Itoa(minKhz),
		dir + "scaling_max_freq":            strconv.Itoa(maxKhz),
	}
}

func armCPUs(midr string, capacity int, cpus ...int) map[string]string {
	tree := map[string]string{}

	for _, cpu := range cpus {
		dir := "devices/system/cpu/cpu" + strconv.Itoa(cpu) + "/"

		tree[dir+"cpu_capacity"] = strconv.Itoa(capacity)
		tree[dir+"regs/identification/midr_el1"] = midr
	}

	return tree
}

func armStatus(affected, related []uint32, minKhz, maxKhz uint64, capacity uint32, coreType string) hardware.CPUScalingStatusSpec {
	return hardware.CPUScalingStatusSpec{
		Driver:                 "cpufreq-dt",
		AffectedCPUs:           affected,
		RelatedCPUs:            related,
		AvailableGovernors:     strings.Fields(armGovernors),
		Governor:               "schedutil",
		CPUInfoMinFrequencyKhz: minKhz,
		CPUInfoMaxFrequencyKhz: maxKhz,
		ScalingMinFrequencyKhz: minKhz,
		ScalingMaxFrequencyKhz: maxKhz,
		CPUCapacity:            capacity,
		CoreType:               coreType,
	}
}

func mergeTrees(trees ...map[string]string) map[string]string {
	merged := map[string]string{}

	for _, tree := range trees {
		maps.Copy(merged, tree)
	}

	return merged
}

type CPUScalingArmSuite struct {
	ctest.DefaultSuite

	tree     map[string]string
	expected map[string]hardware.CPUScalingStatusSpec
}

func (suite *CPUScalingArmSuite) SetupTest() {
	suite.DefaultSuite.SetupTest()

	root := suite.T().TempDir()

	writeSysfs(suite.T(), root, suite.tree)

	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.CPUScalingController{
		SysfsPath: root,
	}))
}

func (suite *CPUScalingArmSuite) TestReportPolicies() {
	assertStatuses(suite, suite.expected)
}

func TestCPUScalingArmSuite(t *testing.T) {
	t.Parallel()

	for name, tc := range armTrees {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			suite.Run(t, &CPUScalingArmSuite{
				Timeout:  10 * time.Second,
				tree:     tc.tree,
				expected: tc.expected,
			})
		})
	}
}
