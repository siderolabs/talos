// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package oom_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/oom"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const sampleInterval = 500 * time.Millisecond

func writeFile(t testing.TB, path, contents string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
}

func writePSI(t testing.TB, dir string, avg10, total float64) {
	t.Helper()

	writeFile(t, filepath.Join(dir, "memory.pressure"), fmt.Sprintf(
		"some avg10=%.2f avg60=0.00 avg300=0.00 total=%d\nfull avg10=%.2f avg60=0.00 avg300=0.00 total=%d\n",
		avg10, int64(total), avg10, int64(total),
	))
}

func writeMemory(t testing.TB, dir string, current int64, maxValue string) {
	t.Helper()

	writeFile(t, filepath.Join(dir, "memory.current"), fmt.Sprintf("%d\n", current))
	writeFile(t, filepath.Join(dir, "memory.max"), maxValue+"\n")
	writeFile(t, filepath.Join(dir, "memory.peak"), fmt.Sprintf("%d\n", current))
}

// writeCgroupTree populates a fake cgroupfs with all the cgroups sampled by the OOM controller.
func writeCgroupTree(t testing.TB, root string, guaranteedPods ...string) {
	t.Helper()

	writePSI(t, root, 1, 1000)

	for _, dir := range append(
		[]string{"init", "system", "podruntime", "kubepods/besteffort", "kubepods/burstable"},
		guaranteedPods...,
	) {
		writePSI(t, filepath.Join(root, dir), 1, 1000)
		writeMemory(t, filepath.Join(root, dir), 1024, "max")
	}

	// cgroupfs control files of the kubepods cgroup itself should be ignored
	writeMemory(t, filepath.Join(root, "kubepods"), 1024, "max")
}

func TestSamplerAllocs(t *testing.T) {
	root := t.TempDir()

	writeCgroupTree(t, root, "kubepods/pod-a", "kubepods/pod-b", "kubepods/pod-c")

	sampler := oom.NewSampler(root)
	t.Cleanup(sampler.Close)

	// warm up: create the state of the Guaranteed pods and the eval context maps
	require.NoError(t, sampler.Sample(sampleInterval))
	sampler.EvalContext()

	allocs := testing.AllocsPerRun(100, func() {
		if err := sampler.Sample(sampleInterval); err != nil {
			t.Fatal(err)
		}
	})

	assert.Zero(t, allocs, "Sample should not allocate in the steady state")

	allocs = testing.AllocsPerRun(100, func() {
		sampler.EvalContext()
	})

	// only boxing of non-zero float64 scalars (memory_*, d_memory_*) into map[string]any allocates
	assert.LessOrEqual(t, allocs, 16.0, "EvalContext should only allocate for scalar values")
}

func TestSamplerRates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	writeCgroupTree(t, root, "kubepods/pod-a")

	sampler := oom.NewSampler(root)

	require.NoError(t, sampler.Sample(sampleInterval))

	evalContext := sampler.EvalContext()

	// no previous sample, so no rate of change
	assert.Equal(t, 1000.0, evalContext["memory_full_total"])
	assert.Equal(t, 0.0, evalContext["d_memory_full_total"])
	assert.Equal(t, 2000.0, evalContext["qos_memory_full_total"].(map[int]float64)[int(runtime.QoSCgroupClassSystem)])
	assert.Equal(t, 0.0, evalContext["d_qos_memory_full_total"].(map[int]float64)[int(runtime.QoSCgroupClassSystem)])
	assert.Equal(t, 1024.0, evalContext["qos_memory_current"].(map[int]float64)[int(runtime.QoSCgroupClassGuaranteed)])
	assert.Equal(t, 0.0, evalContext["qos_memory_max"].(map[int]float64)[int(runtime.QoSCgroupClassGuaranteed)], "max is reported as zero")

	writePSI(t, root, 2, 1500)
	writePSI(t, filepath.Join(root, "system"), 3, 2000)
	writeMemory(t, filepath.Join(root, "kubepods/pod-a"), 2048, "4096")

	// new Guaranteed pod: its first sample shouldn't affect the rate of change
	writePSI(t, filepath.Join(root, "kubepods/pod-b"), 10, 100000)
	writeMemory(t, filepath.Join(root, "kubepods/pod-b"), 1<<20, "max")

	require.NoError(t, sampler.Sample(sampleInterval))

	evalContext = sampler.EvalContext()

	assert.Equal(t, 1500.0, evalContext["memory_full_total"])
	assert.Equal(t, 1000.0, evalContext["d_memory_full_total"]) // (1500 - 1000) / 0.5s
	assert.Equal(t, 2.0, evalContext["d_memory_full_avg10"])    // (2 - 1) / 0.5s
	assert.Equal(t, 3000.0, evalContext["qos_memory_full_total"].(map[int]float64)[int(runtime.QoSCgroupClassSystem)])
	assert.Equal(t, 2000.0, evalContext["d_qos_memory_full_total"].(map[int]float64)[int(runtime.QoSCgroupClassSystem)])

	assert.Equal(t, 101000.0, evalContext["qos_memory_full_total"].(map[int]float64)[int(runtime.QoSCgroupClassGuaranteed)])
	assert.Equal(t, 0.0, evalContext["d_qos_memory_full_total"].(map[int]float64)[int(runtime.QoSCgroupClassGuaranteed)])

	assert.Equal(t, 2048.0+(1<<20), evalContext["qos_memory_current"].(map[int]float64)[int(runtime.QoSCgroupClassGuaranteed)])
	assert.Equal(t, 2048.0, evalContext["d_qos_memory_current"].(map[int]float64)[int(runtime.QoSCgroupClassGuaranteed)]) // (2048 - 1024) / 0.5s
	assert.Equal(t, 4096.0, evalContext["qos_memory_max"].(map[int]float64)[int(runtime.QoSCgroupClassGuaranteed)])

	// pod-a goes away, and its state is dropped
	require.NoError(t, os.RemoveAll(filepath.Join(root, "kubepods/pod-a")))

	require.NoError(t, sampler.Sample(sampleInterval))

	evalContext = sampler.EvalContext()

	assert.Equal(t, 100000.0, evalContext["qos_memory_full_total"].(map[int]float64)[int(runtime.QoSCgroupClassGuaranteed)])
	assert.Equal(t, 0.0, evalContext["d_qos_memory_full_total"].(map[int]float64)[int(runtime.QoSCgroupClassGuaranteed)])
	assert.Equal(t, float64(1<<20), evalContext["qos_memory_current"].(map[int]float64)[int(runtime.QoSCgroupClassGuaranteed)])

	// pod-a comes back (same name), it is treated as a new cgroup
	writeCgroupTree(t, root, "kubepods/pod-a")

	require.NoError(t, sampler.Sample(sampleInterval))

	evalContext = sampler.EvalContext()

	assert.Equal(t, 0.0, evalContext["d_qos_memory_current"].(map[int]float64)[int(runtime.QoSCgroupClassGuaranteed)])
}

func TestSamplerMissingCgroups(t *testing.T) {
	t.Parallel()

	sampler := oom.NewSampler(filepath.Join(t.TempDir(), "does-not-exist"))

	require.NoError(t, sampler.Sample(sampleInterval))

	evalContext := sampler.EvalContext()

	assert.Equal(t, 0.0, evalContext["memory_some_avg10"])

	for _, class := range []runtime.QoSCgroupClass{
		runtime.QoSCgroupClassBesteffort,
		runtime.QoSCgroupClassBurstable,
		runtime.QoSCgroupClassGuaranteed,
		runtime.QoSCgroupClassPodruntime,
		runtime.QoSCgroupClassSystem,
	} {
		assert.Contains(t, evalContext["qos_memory_current"], int(class))
	}
}

func TestSamplerErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		contents  string
		expectErr string
	}{
		{
			name:      "missing full",
			contents:  "some avg10=0.00 avg60=0.00 avg300=0.00 total=0\n",
			expectErr: "cannot find memory pressure entry: type: full, span: avg10",
		},
		{
			name:      "missing span",
			contents:  "some avg10=0.00 avg60=0.00 avg300=0.00 total=0\nfull avg10=0.00 avg60=0.00 total=0\n",
			expectErr: "cannot find memory pressure entry: type: full, span: avg300",
		},
		{
			name:      "max",
			contents:  "some avg10=max avg60=0.00 avg300=0.00 total=0\nfull avg10=0.00 avg60=0.00 avg300=0.00 total=0\n",
			expectErr: "PSI is not defined: type: some, span: avg10",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()

			writeFile(t, filepath.Join(root, "memory.pressure"), test.contents)

			assert.EqualError(t, oom.NewSampler(root).Sample(sampleInterval), test.expectErr)
		})
	}

	assert.Error(t, oom.NewSampler(t.TempDir()).Sample(0))
}

// openFile is a file kept open by the process.
type openFile struct {
	// path relative to the root
	path string
	fd   int

	// current is true if the open file is the file at the path (i.e. it wasn't removed or replaced).
	current bool
}

// openFiles returns the files under root kept open by the process.
func openFiles(t *testing.T, root string) []openFile {
	t.Helper()

	root, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)

	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)

	var files []openFile

	for _, entry := range entries {
		fdPath := filepath.Join("/proc/self/fd", entry.Name())

		target, err := os.Readlink(fdPath)
		if err != nil {
			continue // closed concurrently
		}

		// removed files are reported with a suffix (except on kernfs, which reports the original path)
		path := strings.TrimSuffix(target, " (deleted)")

		if !strings.HasPrefix(path, root+string(filepath.Separator)) {
			continue
		}

		fd, err := strconv.Atoi(entry.Name())
		require.NoError(t, err)

		// stat via /proc/self/fd returns the open file, even if it was removed
		openStat, err := os.Stat(fdPath)
		if err != nil {
			continue // closed concurrently
		}

		pathStat, err := os.Stat(path)

		rel, relErr := filepath.Rel(root, path)
		require.NoError(t, relErr)

		files = append(files, openFile{
			path:    rel,
			fd:      fd,
			current: err == nil && os.SameFile(openStat, pathStat),
		})
	}

	return files
}

// openFilesAt returns the open files with the specified path (relative to the root).
func openFilesAt(t *testing.T, root, path string) []openFile {
	t.Helper()

	var files []openFile

	for _, f := range openFiles(t, root) {
		if f.path == path {
			files = append(files, f)
		}
	}

	return files
}

func TestSamplerOpenFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	writeCgroupTree(t, root, "kubepods/pod-a", "kubepods/pod-b")

	sampler := oom.NewSampler(root)
	t.Cleanup(sampler.Close)

	assert.Empty(t, openFiles(t, root))

	require.NoError(t, sampler.Sample(sampleInterval))

	// root cgroup: memory.pressure; QoS class cgroups and Guaranteed pods: memory.{pressure,current,max,peak}
	const classFiles = 1 + 5*4

	files := openFiles(t, root)
	assert.Len(t, files, classFiles+2*4)

	for _, f := range files {
		assert.True(t, f.current, f.path)
	}

	const systemPressure = "system/memory.pressure"

	pressureFiles := openFilesAt(t, root, systemPressure)
	require.Len(t, pressureFiles, 1)

	// the files are kept open, and the updated contents are read on the next sample
	writePSI(t, filepath.Join(root, "system"), 3, 2000)

	require.NoError(t, sampler.Sample(sampleInterval))

	assert.Equal(t, pressureFiles, openFilesAt(t, root, systemPressure))

	assert.Equal(t, 3000.0, sampler.EvalContext()["qos_memory_full_total"].(map[int]float64)[int(runtime.QoSCgroupClassSystem)])

	// the files of a removed Guaranteed pod are closed
	const podCurrent = "kubepods/pod-a/memory.current"

	assert.Len(t, openFilesAt(t, root, podCurrent), 1)

	require.NoError(t, os.RemoveAll(filepath.Join(root, "kubepods/pod-a")))

	require.NoError(t, sampler.Sample(sampleInterval))

	assert.Empty(t, openFilesAt(t, root, podCurrent))
	assert.Len(t, openFiles(t, root), classFiles+4)

	sampler.Close()

	assert.Empty(t, openFiles(t, root))

	// the files are re-opened after Close
	require.NoError(t, sampler.Sample(sampleInterval))

	assert.Len(t, openFiles(t, root), classFiles+4)
}

// testCgroup creates a cgroup for the test under the cgroup of the test process.
//
// The test is skipped if the cgroup can't be created, run it with `systemd-run --user --scope -p Delegate=yes`.
func testCgroup(t *testing.T) string {
	t.Helper()

	contents, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		t.Skipf("cannot read the cgroup of the process: %v", err)
	}

	self, ok := strings.CutPrefix(strings.TrimSpace(string(contents)), "0::")
	if !ok {
		t.Skipf("not on cgroups v2: %q", contents)
	}

	dir := filepath.Join(constants.CgroupMountPath, self, "oom-sampler-test-"+strconv.Itoa(os.Getpid()))

	if err = os.Mkdir(dir, 0o755); err != nil {
		t.Skipf("cannot create a cgroup (run with `systemd-run --user --scope -p Delegate=yes`): %v", err)
	}

	t.Cleanup(func() {
		assert.NoError(t, os.Remove(dir))
	})

	if _, err = os.Stat(filepath.Join(dir, "memory.pressure")); err != nil {
		t.Skipf("PSI is not available: %v", err)
	}

	return dir
}

func TestSamplerCgroupfs(t *testing.T) {
	t.Parallel()

	root := testCgroup(t)
	system := filepath.Join(root, constants.CgroupSystem)

	require.NoError(t, os.Mkdir(system, 0o755))

	t.Cleanup(func() {
		os.Remove(system) //nolint:errcheck
	})

	sampler := oom.NewSampler(root)
	t.Cleanup(sampler.Close)

	const pressure = constants.CgroupSystem + "/memory.pressure"

	assertOpen := func() {
		t.Helper()

		files := openFilesAt(t, root, pressure)
		if assert.Len(t, files, 1) {
			assert.True(t, files[0].current, "the open file should be the file of the current cgroup")
		}
	}

	require.NoError(t, sampler.Sample(sampleInterval))

	assertOpen()

	// the cgroup is removed: the read fails (ENODEV), and the file is closed
	require.NoError(t, os.Remove(system))

	require.NoError(t, sampler.Sample(sampleInterval))

	assert.Empty(t, openFilesAt(t, root, pressure))

	// the cgroup is created again
	require.NoError(t, os.Mkdir(system, 0o755))

	require.NoError(t, sampler.Sample(sampleInterval))

	assertOpen()

	// the cgroup is re-created between the samples: the file of the removed cgroup is replaced
	require.NoError(t, os.Remove(system))
	require.NoError(t, os.Mkdir(system, 0o755))

	files := openFilesAt(t, root, pressure)
	require.Len(t, files, 1)
	assert.False(t, files[0].current)

	require.NoError(t, sampler.Sample(sampleInterval))

	assertOpen()
}

func BenchmarkSampler(b *testing.B) {
	root := b.TempDir()

	writeCgroupTree(b, root, "kubepods/pod-a", "kubepods/pod-b", "kubepods/pod-c")

	sampler := oom.NewSampler(root)
	b.Cleanup(sampler.Close)

	b.ReportAllocs()

	for b.Loop() {
		if err := sampler.Sample(sampleInterval); err != nil {
			b.Fatal(err)
		}

		sampler.EvalContext()
	}
}
