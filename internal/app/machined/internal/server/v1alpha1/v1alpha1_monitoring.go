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

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/prometheus/procfs"

	"github.com/talos-systems/talos/pkg/machinery/api/machine"
)

// Hostname implements the machine.MachineServer interface.
func (s *Server) Hostname(ctx context.Context, in *empty.Empty) (*machine.HostnameResponse, error) {
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
func (s *Server) LoadAvg(ctx context.Context, in *empty.Empty) (*machine.LoadAvgResponse, error) {
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
func (s *Server) SystemStat(ctx context.Context, in *empty.Empty) (*machine.SystemStatResponse, error) {
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

	translateListOfCPUStat := func(in []procfs.CPUStat) []*machine.CPUStat {
		res := make([]*machine.CPUStat, len(in))

		for i := range in {
			res[i] = translateCPUStat(in[i])
		}

		return res
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

// CPUInfo implements the machine.MachineServer interface.
func (s *Server) CPUInfo(ctx context.Context, in *empty.Empty) (*machine.CPUInfoResponse, error) {
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

	resp := machine.CPUsInfo{
		CpuInfo: make([]*machine.CPUInfo, len(info)),
	}

	for i := range info {
		resp.CpuInfo[i] = translateCPUInfo(info[i])
	}

	reply := &machine.CPUInfoResponse{
		Messages: []*machine.CPUsInfo{
			&resp,
		},
	}

	return reply, nil
}

// NetworkDeviceStats implements the machine.MachineServer interface.
func (s *Server) NetworkDeviceStats(ctx context.Context, in *empty.Empty) (*machine.NetworkDeviceStatsResponse, error) {
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

	resp := machine.NetworkDeviceStats{
		Devices: make([]*machine.NetDev, len(info)),
		Total:   translateNetDevLine(info.Total()),
	}

	i := 0

	for _, line := range info {
		resp.Devices[i] = translateNetDevLine(line)
		i++ //nolint:wastedassign
	}

	reply := &machine.NetworkDeviceStatsResponse{
		Messages: []*machine.NetworkDeviceStats{
			&resp,
		},
	}

	return reply, nil
}

// DiskStats implements the machine.MachineServer interface.
func (s *Server) DiskStats(ctx context.Context, in *empty.Empty) (*machine.DiskStatsResponse, error) {
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
