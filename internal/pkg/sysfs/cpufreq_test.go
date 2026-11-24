// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysfs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/siderolabs/talos/internal/pkg/sysfs"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

func writeAttrs(t *testing.T, dir string, attrs map[string]string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(dir, 0o755))

	for name, value := range attrs {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(value+"\n"), 0o644))
	}
}

func cpufreqAttrs(cur, governor string) map[string]string {
	return map[string]string{
		"cpuinfo_cur_freq": cur,
		"cpuinfo_min_freq": "1500000",
		"cpuinfo_max_freq": "3700000",
		"cpuinfo_avg_freq": "garbage",
		"scaling_governor": governor,
		"scaling_cur_freq": cur,
	}
}

func TestCPUFreqStats(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cpuDir := filepath.Join(root, "devices/system/cpu")

	writeAttrs(t, cpuDir, map[string]string{"online": "0-2,4,10"})

	// cpu0: full set of attributes
	writeAttrs(t, filepath.Join(cpuDir, "cpu0/cpufreq"), cpufreqAttrs("2400000", "performance"))
	// cpu1: no cpufreq policy
	require.NoError(t, os.MkdirAll(filepath.Join(cpuDir, "cpu1"), 0o755))
	// cpu2: current frequency can't be read
	writeAttrs(t, filepath.Join(cpuDir, "cpu2/cpufreq"), cpufreqAttrs("<unknown>", "performance"))
	// cpu3: offline, so skipped
	writeAttrs(t, filepath.Join(cpuDir, "cpu3/cpufreq"), cpufreqAttrs("1000000", "powersave"))
	// cpu4: no governor
	attrs := cpufreqAttrs("3000000", "")
	delete(attrs, "scaling_governor")
	writeAttrs(t, filepath.Join(cpuDir, "cpu4/cpufreq"), attrs)
	// cpu10: sorted numerically after cpu4
	writeAttrs(t, filepath.Join(cpuDir, "cpu10/cpufreq"), cpufreqAttrs("1600000", "schedutil"))

	stats, err := sysfs.CPUFreqStats(root)
	require.NoError(t, err)

	expected := []*machine.CPUFreqStats{
		{CurrentFrequency: 2400000, MinimumFrequency: 1500000, MaximumFrequency: 3700000, Governor: "performance"},
		{},
		{},
		{CurrentFrequency: 3000000, MinimumFrequency: 1500000, MaximumFrequency: 3700000},
		{CurrentFrequency: 1600000, MinimumFrequency: 1500000, MaximumFrequency: 3700000, Governor: "schedutil"},
	}

	require.Len(t, stats, len(expected))

	for i := range expected {
		assert.True(t, proto.Equal(expected[i], stats[i]), "cpu index %d: expected %v, got %v", i, expected[i], stats[i])
	}
}

func TestCPUFreqStatsNoOnline(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	_, err := sysfs.CPUFreqStats(root)
	require.ErrorIs(t, err, fs.ErrNotExist)

	writeAttrs(t, filepath.Join(root, "devices/system/cpu"), map[string]string{"online": ""})

	_, err = sysfs.CPUFreqStats(root)
	require.Error(t, err)
}
