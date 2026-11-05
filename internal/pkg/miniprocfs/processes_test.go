// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package miniprocfs_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/miniprocfs"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

func TestLive(t *testing.T) {
	processes, err := miniprocfs.NewProcesses()
	require.NoError(t, err)

	count := 0

	for {
		proc, err := processes.Next()
		require.NoError(t, err)

		if proc == nil {
			break
		}

		count++
	}

	assert.Greater(t, count, 0)

	assert.NoError(t, processes.Close())
}

func TestMock(t *testing.T) {
	processes, err := miniprocfs.NewProcessesWithPath("testdata/")
	require.NoError(t, err)

	gold, err := procfs.NewFS("testdata/")
	require.NoError(t, err)

	for {
		proc, err := processes.Next()
		require.NoError(t, err)

		if proc == nil {
			break
		}

		goldInfo, err := gold.Proc(int(proc.Pid))
		require.NoError(t, err)

		goldExecutable, err := goldInfo.Executable()
		require.NoError(t, err)

		assert.Equal(t, goldExecutable, proc.Executable)

		goldCommand, err := goldInfo.Comm()
		require.NoError(t, err)

		assert.Equal(t, goldCommand, proc.Command)

		goldCmdline, err := goldInfo.CmdLine()
		require.NoError(t, err)

		assert.Equal(t, strings.Join(goldCmdline, " "), proc.Args)

		goldStat, err := goldInfo.Stat()
		require.NoError(t, err)

		assert.EqualValues(t, goldStat.PPID, proc.Ppid)
		assert.EqualValues(t, goldStat.NumThreads, proc.Threads)
		assert.EqualValues(t, goldStat.CPUTime(), proc.CpuTime)
		assert.EqualValues(t, goldStat.VirtualMemory(), proc.VirtualMemory)
		assert.EqualValues(t, goldStat.ResidentMemory(), proc.ResidentMemory)
	}

	assert.NoError(t, processes.Close())
}

func BenchmarkPrometheusProcfs(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		var resp []*machine.ProcessInfo

		procs, err := procfs.AllProcs()
		require.NoError(b, err)

		for _, proc := range procs {
			executable, err := proc.Executable()
			if err != nil {
				continue
			}

			command, err := proc.Comm()
			if err != nil {
				continue
			}

			args, err := proc.CmdLine()
			if err != nil {
				continue
			}

			stats, err := proc.Stat()
			if err != nil {
				continue
			}

			resp = append(resp, &machine.ProcessInfo{
				Pid:            int32(proc.PID),
				Ppid:           int32(stats.PPID),
				State:          stats.State,
				Threads:        int32(stats.NumThreads),
				CpuTime:        stats.CPUTime(),
				VirtualMemory:  uint64(stats.VirtualMemory()),
				ResidentMemory: uint64(stats.ResidentMemory()),
				Command:        command,
				Executable:     executable,
				Args:           strings.Join(args, " "),
			})
		}

		_ = resp
	}
}

func BenchmarkProcesses(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		var resp []*machine.ProcessInfo

		processes, err := miniprocfs.NewProcesses()
		require.NoError(b, err)

		for {
			proc, err := processes.Next()
			require.NoError(b, err)

			if proc == nil {
				break
			}

			resp = append(resp, proc)
		}

		_ = resp

		assert.NoError(b, processes.Close())
	}
}

func TestMalformed(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	writeProc := func(pid, cmdline, stat string) {
		dir := filepath.Join(root, pid)

		require.NoError(t, os.Mkdir(dir, 0o755))
		require.NoError(t, os.Symlink("/usr/bin/test", filepath.Join(dir, "exe")))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "comm"), []byte("test\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "stat"), []byte(stat), 0o644))
	}

	stat, err := os.ReadFile("testdata/1920080/stat")
	require.NoError(t, err)

	// valid
	writeProc("100", "test\x00--flag\x00", string(stat))
	// empty command line
	writeProc("200", "", string(stat))
	// truncated stat
	writeProc("300", "test\x00", "300 (test) S 1 2 3")
	// invalid number in stat
	writeProc("400", "test\x00", strings.Replace(string(stat), " 1920079 ", " abc ", 1))
	// stat without the command
	writeProc("500", "test\x00", "500 test S 1")
	// not a process
	writeProc("self", "test\x00", string(stat))
	writeProc("99999999999", "test\x00", string(stat))

	processes, err := miniprocfs.NewProcessesWithPath(root)
	require.NoError(t, err)

	got := map[int32]*machine.ProcessInfo{}

	for {
		proc, err := processes.Next()
		require.NoError(t, err)

		if proc == nil {
			break
		}

		got[proc.Pid] = proc
	}

	require.NoError(t, processes.Close())

	assert.Len(t, got, 2)

	require.Contains(t, got, int32(100))
	assert.Equal(t, "test --flag", got[100].Args)
	assert.Equal(t, "test", got[100].Command)
	assert.Equal(t, "/usr/bin/test", got[100].Executable)
	assert.Equal(t, "S", got[100].State)
	assert.EqualValues(t, 1920079, got[100].Ppid)
	assert.EqualValues(t, 2, got[100].Threads)

	require.Contains(t, got, int32(200))
	assert.Empty(t, got[200].Args)

	// closed iterator
	_, err = processes.Next()
	assert.ErrorIs(t, err, os.ErrClosed)
	assert.ErrorIs(t, processes.Close(), os.ErrClosed)
}

func TestAllocs(t *testing.T) {
	walk := func() int {
		processes, err := miniprocfs.NewProcessesWithPath("testdata/")
		if err != nil {
			t.Fatal(err)
		}

		count := 0

		for {
			proc, err := processes.Next()
			if err != nil {
				t.Fatal(err)
			}

			if proc == nil {
				break
			}

			count++
		}

		if err = processes.Close(); err != nil {
			t.Fatal(err)
		}

		return count
	}

	count := walk()
	require.Equal(t, 2, count)

	allocs := testing.AllocsPerRun(100, func() { walk() })

	// the iterator, a copy of the root path and the cleanup, and for each process: process info, executable, command, args
	assert.LessOrEqual(t, allocs, float64(4+4*count))
}

// openFiles returns the number of file descriptors of the process which refer to the path.
func openFiles(t *testing.T, path string) int {
	t.Helper()

	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)

	n := 0

	for _, entry := range entries {
		// the fd might be closed concurrently, so ignore errors
		if target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name())); err == nil && target == path {
			n++
		}
	}

	return n
}

func TestNotClosed(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	func() {
		_, err := miniprocfs.NewProcessesWithPath(root)
		require.NoError(t, err)
	}()

	require.Equal(t, 1, openFiles(t, root))

	// the iterator which is not closed releases the directory when garbage collected
	assert.Eventually(t, func() bool {
		runtime.GC()

		return openFiles(t, root) == 0
	}, 5*time.Second, 10*time.Millisecond)
}

func TestClose(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	processes, err := miniprocfs.NewProcessesWithPath(root)
	require.NoError(t, err)

	assert.Equal(t, 1, openFiles(t, root))

	require.NoError(t, processes.Close())

	assert.Equal(t, 0, openFiles(t, root))
}
