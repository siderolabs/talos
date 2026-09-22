// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hardwarectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// writeSysfs materializes a fake sysfs tree, newline-terminating values as the kernel does.
func writeSysfs(t *testing.T, root string, files map[string]string) {
	t.Helper()

	for path, contents := range files {
		full := filepath.Join(root, path)

		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(contents+"\n"), 0o644))
	}
}

func readPolicyAttr(t *testing.T, root, policy, name string) string {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(root, "devices/system/cpu/cpufreq", policy, name))
	require.NoError(t, err)

	return strings.TrimSpace(string(contents))
}

// cpufreqTree covers the shapes a cpufreq driver can present:
//
//   - policy0 mirrors a real intel_cpufreq box: one CPU per policy, no EPP, no base frequency;
//   - policy1 drives several CPUs at once, as ARM and many server parts do;
//   - policy2 is HWP-style, with EPP and a base frequency;
//   - policy3 is offline, so it has related CPUs but no affected ones.
func cpufreqTree() map[string]string {
	return map[string]string{
		"devices/system/cpu/cpufreq/policy0/scaling_driver":              "intel_cpufreq",
		"devices/system/cpu/cpufreq/policy0/affected_cpus":               "0",
		"devices/system/cpu/cpufreq/policy0/related_cpus":                "0",
		"devices/system/cpu/cpufreq/policy0/scaling_available_governors": "conservative ondemand userspace powersave performance schedutil",
		"devices/system/cpu/cpufreq/policy0/scaling_governor":            "schedutil",
		"devices/system/cpu/cpufreq/policy0/scaling_min_freq":            "1200000",
		"devices/system/cpu/cpufreq/policy0/scaling_max_freq":            "3300000",
		"devices/system/cpu/cpufreq/policy0/cpuinfo_min_freq":            "1200000",
		"devices/system/cpu/cpufreq/policy0/cpuinfo_max_freq":            "3300000",
		"devices/system/cpu/cpufreq/policy0/scaling_cur_freq":            "2712000",

		"devices/system/cpu/cpufreq/policy1/scaling_driver":              "acpi-cpufreq",
		"devices/system/cpu/cpufreq/policy1/affected_cpus":               "1-3",
		"devices/system/cpu/cpufreq/policy1/related_cpus":                "1-3",
		"devices/system/cpu/cpufreq/policy1/scaling_available_governors": "ondemand performance",
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
		"devices/system/cpu/cpufreq/policy2/energy_performance_available_preferences": "default performance balance_performance balance_power power",
		"devices/system/cpu/cpufreq/policy2/scaling_min_freq":                         "400000",
		"devices/system/cpu/cpufreq/policy2/scaling_max_freq":                         "3900000",
		"devices/system/cpu/cpufreq/policy2/cpuinfo_min_freq":                         "400000",
		"devices/system/cpu/cpufreq/policy2/cpuinfo_max_freq":                         "3900000",
		"devices/system/cpu/cpufreq/policy2/base_frequency":                           "2100000",

		"devices/system/cpu/cpufreq/policy3/scaling_driver":              "acpi-cpufreq",
		"devices/system/cpu/cpufreq/policy3/affected_cpus":               "",
		"devices/system/cpu/cpufreq/policy3/related_cpus":                "8",
		"devices/system/cpu/cpufreq/policy3/scaling_available_governors": "performance",
		"devices/system/cpu/cpufreq/policy3/scaling_governor":            "performance",

		"devices/cpu_core/cpus": "0-2",
		"devices/cpu_atom/cpus": "3-4,8",

		"devices/system/cpu/cpu0/cpu_capacity": "1024",
		"devices/system/cpu/cpu4/cpu_capacity": "512",
	}
}

type CPUScalingSuite struct {
	ctest.DefaultSuite

	root        string
	reconcileCh chan struct{}
}

func (suite *CPUScalingSuite) SetupTest() {
	suite.DefaultSuite.SetupTest()

	suite.root = suite.T().TempDir()
	suite.reconcileCh = make(chan struct{}, 1)

	writeSysfs(suite.T(), suite.root, cpufreqTree())

	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.CPUScalingController{
		SysfsPath:   suite.root,
		ReconcileCh: suite.reconcileCh,
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
		"policy3": {
			Driver:             "acpi-cpufreq",
			RelatedCPUs:        []uint32{8},
			AvailableGovernors: []string{"performance"},
			Governor:           "performance",
			CoreType:           hardware.CoreTypeEfficiency,
		},
	}

	for id, spec := range expected {
		ctest.AssertResource(suite, id, func(r *hardware.CPUScalingStatus, asrt *assert.Assertions) {
			asrt.Equal(spec, *r.TypedSpec())
		})
	}
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
	})

	assert.Equal(suite.T(), "1400000", readPolicyAttr(suite.T(), suite.root, "policy0", "scaling_min_freq"))
	assert.Equal(suite.T(), "2800000", readPolicyAttr(suite.T(), suite.root, "policy0", "scaling_max_freq"))
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
