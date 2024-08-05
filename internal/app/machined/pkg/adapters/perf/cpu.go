// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package perf

import (
	"github.com/prometheus/procfs"

	"github.com/siderolabs/talos/pkg/machinery/resources/perf"
)

// CPU adapter provides conversion from procfs.
//
//nolint:revive,golint
func CPU(r *perf.CPU) cpu {
	return cpu{
		CPU: r,
	}
}

type cpu struct {
	*perf.CPU
}

// Update current CPU snapshot.
func (a cpu) Update(stat *procfs.Stat) {
	translateCPUStat := func(in procfs.CPUStat) perf.CPUStat {
		return perf.CPUStat{
			User:      in.User,
			Nice:      in.Nice,
			System:    in.System,
			Idle:      in.Idle,
			Iowait:    in.Iowait,
			Irq:       in.IRQ,
			SoftIrq:   in.SoftIRQ,
			Steal:     in.Steal,
			Guest:     in.Guest,
			GuestNice: in.GuestNice,
		}
	}

	translateListOfCPUStat := func(in map[int64]procfs.CPUStat) []perf.CPUStat {
		maxCore := int64(-1)

		for core := range in {
			maxCore = max(maxCore, core)
		}

		slc := make([]perf.CPUStat, maxCore+1)

		for core, stat := range in {
			slc[core] = translateCPUStat(stat)
		}

		return slc
	}

	*a.CPU.TypedSpec() = perf.CPUSpec{
		CPUTotal:        translateCPUStat(stat.CPUTotal),
		CPU:             translateListOfCPUStat(stat.CPU),
		IRQTotal:        stat.IRQTotal,
		ContextSwitches: stat.ContextSwitches,
		ProcessCreated:  stat.ProcessCreated,
		ProcessRunning:  stat.ProcessesRunning,
		ProcessBlocked:  stat.ProcessesBlocked,
		SoftIrqTotal:    stat.SoftIRQTotal,
	}
}
