// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/procfs"
	"github.com/prometheus/procfs/sysfs"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// Hostname implements the machine.MachineServer interface.
func (s *Server) Hostname(ctx context.Context, in *emptypb.Empty) (*machine.HostnameResponse, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	reply := &machine.HostnameResponse{
		Messages: []*machine.Hostname{
			{
				Hostname: hostname,
			},
		},
	}

	return reply, nil
}

// LoadAvg implements the machine.MachineServer interface.
func (s *Server) LoadAvg(ctx context.Context, in *emptypb.Empty) (*machine.LoadAvgResponse, error) {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	loadAvg, err := fs.LoadAvg()
	if err != nil {
		return nil, err
	}

	reply := &machine.LoadAvgResponse{
		Messages: []*machine.LoadAvg{
			{
				Load1:  loadAvg.Load1,
				Load5:  loadAvg.Load5,
				Load15: loadAvg.Load15,
			},
		},
	}

	return reply, nil
}

// SystemStat implements the machine.MachineServer interface.
func (s *Server) SystemStat(ctx context.Context, in *emptypb.Empty) (*machine.SystemStatResponse, error) {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	stat, err := fs.Stat()
	if err != nil {
		return nil, err
	}

	translateCPUStat := func(in procfs.CPUStat) *machine.CPUStat {
		return &machine.CPUStat{
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

	translateListOfCPUStat := func(in map[int64]procfs.CPUStat) []*machine.CPUStat {
		maxCore := int64(-1)

		for core := range in {
			maxCore = max(maxCore, core)
		}

		slc := make([]*machine.CPUStat, maxCore+1)

		for core, stat := range in {
			slc[core] = translateCPUStat(stat)
		}

		return slc
	}

	translateSoftIRQ := func(in procfs.SoftIRQStat) *machine.SoftIRQStat {
		return &machine.SoftIRQStat{
			Hi:          in.Hi,
			Timer:       in.Timer,
			NetTx:       in.NetTx,
			NetRx:       in.NetRx,
			Block:       in.Block,
			BlockIoPoll: in.BlockIoPoll,
			Tasklet:     in.Tasklet,
			Sched:       in.Sched,
			Hrtimer:     in.Hrtimer,
			Rcu:         in.Rcu,
		}
	}

	reply := &machine.SystemStatResponse{
		Messages: []*machine.SystemStat{
			{
				BootTime:        stat.BootTime,
				CpuTotal:        translateCPUStat(stat.CPUTotal),
				Cpu:             translateListOfCPUStat(stat.CPU),
				IrqTotal:        stat.IRQTotal,
				Irq:             stat.IRQ,
				ContextSwitches: stat.ContextSwitches,
				ProcessCreated:  stat.ProcessCreated,
				ProcessRunning:  stat.ProcessesRunning,
				ProcessBlocked:  stat.ProcessesBlocked,
				SoftIrqTotal:    stat.SoftIRQTotal,
				SoftIrq:         translateSoftIRQ(stat.SoftIRQ),
			},
		},
	}

	return reply, nil
}

// CPUFreqStats implements the machine.MachineServer interface.
func (s *Server) CPUFreqStats(ctx context.Context, in *emptypb.Empty) (*machine.CPUFreqStatsResponse, error) {
	fs, err := sysfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	systemCpufreqStats, err := fs.SystemCpufreq()
	if err != nil {
		return nil, err
	}

	translateCPUFreqStats := func(in sysfs.SystemCPUCpufreqStats) *machine.CPUFreqStats {
		if in.CpuinfoCurrentFrequency == nil || in.CpuinfoMinimumFrequency == nil || in.CpuinfoMaximumFrequency == nil {
			return &machine.CPUFreqStats{}
		}

		return &machine.CPUFreqStats{
			CurrentFrequency: *in.CpuinfoCurrentFrequency,
			MinimumFrequency: *in.CpuinfoMinimumFrequency,
			MaximumFrequency: *in.CpuinfoMaximumFrequency,
			Governor:         in.Governor,
		}
	}

	reply := &machine.CPUFreqStatsResponse{
		Messages: []*machine.CPUsFreqStats{
			{
				CpuFreqStats: xslices.Map(systemCpufreqStats, translateCPUFreqStats),
			},
		},
	}

	return reply, nil
}

// CPUInfo implements the machine.MachineServer interface.
func (s *Server) CPUInfo(ctx context.Context, in *emptypb.Empty) (*machine.CPUInfoResponse, error) {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	info, err := fs.CPUInfo()
	if err != nil {
		return nil, err
	}

	translateCPUInfo := func(in procfs.CPUInfo) *machine.CPUInfo {
		return &machine.CPUInfo{
			Processor:       uint32(in.Processor),
			VendorId:        in.VendorID,
			CpuFamily:       in.CPUFamily,
			Model:           in.Model,
			ModelName:       in.ModelName,
			Stepping:        in.Stepping,
			Microcode:       in.Microcode,
			CpuMhz:          in.CPUMHz,
			CacheSize:       in.CacheSize,
			PhysicalId:      in.PhysicalID,
			Siblings:        uint32(in.Siblings),
			CoreId:          in.CoreID,
			ApicId:          in.APICID,
			InitialApicId:   in.InitialAPICID,
			Fpu:             in.FPU,
			FpuException:    in.FPUException,
			CpuIdLevel:      uint32(in.CPUIDLevel),
			Wp:              in.WP,
			Flags:           in.Flags,
			Bugs:            in.Bugs,
			BogoMips:        in.BogoMips,
			ClFlushSize:     uint32(in.CLFlushSize),
			CacheAlignment:  uint32(in.CacheAlignment),
			AddressSizes:    in.AddressSizes,
			PowerManagement: in.PowerManagement,
		}
	}

	reply := &machine.CPUInfoResponse{
		Messages: []*machine.CPUsInfo{
			{
				CpuInfo: xslices.Map(info, translateCPUInfo),
			},
		},
	}

	return reply, nil
}

// NetworkDeviceStats implements the machine.MachineServer interface.
func (s *Server) NetworkDeviceStats(ctx context.Context, in *emptypb.Empty) (*machine.NetworkDeviceStatsResponse, error) {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	info, err := fs.NetDev()
	if err != nil {
		return nil, err
	}

	translateNetDevLine := func(in procfs.NetDevLine) *machine.NetDev {
		return &machine.NetDev{
			Name:         in.Name,
			RxBytes:      in.RxBytes,
			RxPackets:    in.RxPackets,
			RxErrors:     in.RxErrors,
			RxDropped:    in.RxDropped,
			RxFifo:       in.RxFIFO,
			RxFrame:      in.RxFrame,
			RxCompressed: in.RxCompressed,
			RxMulticast:  in.RxMulticast,
			TxBytes:      in.TxBytes,
			TxPackets:    in.TxPackets,
			TxErrors:     in.TxErrors,
			TxDropped:    in.TxDropped,
			TxFifo:       in.TxFIFO,
			TxCollisions: in.TxCollisions,
			TxCarrier:    in.TxCarrier,
			TxCompressed: in.TxCompressed,
		}
	}

	reply := &machine.NetworkDeviceStatsResponse{
		Messages: []*machine.NetworkDeviceStats{
			{
				Devices: maps.ValuesFunc(info, translateNetDevLine),
				Total:   translateNetDevLine(info.Total()),
			},
		},
	}

	return reply, nil
}

// DiskStats implements the machine.MachineServer interface.
func (s *Server) DiskStats(ctx context.Context, in *emptypb.Empty) (*machine.DiskStatsResponse, error) {
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, err
	}

	defer f.Close() //nolint:errcheck

	resp := machine.DiskStats{
		Devices: []*machine.DiskStat{},
		Total:   &machine.DiskStat{},
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		if len(fields) < 18 {
			continue
		}

		values := make([]uint64, 15)
		for i := range values {
			values[i], err = strconv.ParseUint(fields[3+i], 10, 64)
			if err != nil {
				return nil, err
			}
		}

		stat := &machine.DiskStat{
			Name:             fields[2],
			ReadCompleted:    values[0],
			ReadMerged:       values[1],
			ReadSectors:      values[2],
			ReadTimeMs:       values[3],
			WriteCompleted:   values[4],
			WriteMerged:      values[5],
			WriteSectors:     values[6],
			WriteTimeMs:      values[7],
			IoInProgress:     values[8],
			IoTimeMs:         values[9],
			IoTimeWeightedMs: values[10],
			DiscardCompleted: values[11],
			DiscardMerged:    values[12],
			DiscardSectors:   values[13],
			DiscardTimeMs:    values[14],
		}

		resp.Devices = append(resp.Devices, stat)

		resp.Total.ReadCompleted += stat.ReadCompleted
		resp.Total.ReadMerged += stat.ReadMerged
		resp.Total.ReadSectors += stat.ReadSectors
		resp.Total.ReadTimeMs += stat.ReadTimeMs
		resp.Total.WriteCompleted += stat.WriteCompleted
		resp.Total.WriteMerged += stat.WriteMerged
		resp.Total.WriteSectors += stat.WriteSectors
		resp.Total.WriteTimeMs += stat.WriteTimeMs
		resp.Total.IoInProgress += stat.IoInProgress
		resp.Total.IoTimeMs += stat.IoTimeMs
		resp.Total.IoTimeWeightedMs += stat.IoTimeWeightedMs
		resp.Total.DiscardCompleted += stat.DiscardCompleted
		resp.Total.DiscardMerged += stat.DiscardMerged
		resp.Total.DiscardSectors += stat.DiscardSectors
		resp.Total.DiscardTimeMs += stat.DiscardTimeMs
	}

	if err = scanner.Err(); err != nil {
		return nil, err
	}

	reply := &machine.DiskStatsResponse{
		Messages: []*machine.DiskStats{
			&resp,
		},
	}

	return reply, nil
}
