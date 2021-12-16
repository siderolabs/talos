// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package miniprocfs_test

import (
	"strings"
	"testing"

	"github.com/prometheus/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/internal/pkg/miniprocfs"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
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

	for i := 0; i < b.N; i++ {
		resp := []*machine.ProcessInfo{}

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

	for i := 0; i < b.N; i++ {
		resp := []*machine.ProcessInfo{}

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
