// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package smart

import (
	"fmt"

	smartlib "github.com/anatol/smart.go"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// SMART attribute IDs.
const (
	AttrReallocatedSectorCount      = 5
	AttrReportedUncorrectableErrors = 187
	AttrCurrentPendingSectorCount   = 197
	AttrOfflineUncorrectableCount   = 198
	AttrWearLevelingCount           = 177
)

// Result holds collected disk health data.
type Result struct {
	Source             block.DiskHealthSource
	Status             block.DiskHealthStatusValue
	TemperatureCelsius int32
	PowerOnHours       uint64
	PowerCycles        uint64
	NVMe               *block.DiskHealthNVMeDetails
	ATA                *block.DiskHealthATADetails
	Error              string
}

// Collector defines the interface for collecting disk health data.
type Collector interface {
	Collect(devPath string) Result
}

// RealCollector uses the actual smart.go library.
type RealCollector struct{}

// Collect opens a device and reads health data.
func (c *RealCollector) Collect(devPath string) Result {
	dev, err := smartlib.Open(devPath)
	if err != nil {
		return Result{
			Source: block.DiskHealthSourceUnsupported,
			Status: block.DiskHealthStatusValueUnknown,
			Error:  fmt.Sprintf("failed to open device: %v", err),
		}
	}

	defer dev.Close() //nolint:errcheck

	switch d := dev.(type) {
	case *smartlib.NVMeDevice:
		return collectNVMe(d)
	case *smartlib.SataDevice:
		return collectATA(d)
	default:
		return Result{
			Source: block.DiskHealthSourceUnsupported,
			Status: block.DiskHealthStatusValueUnknown,
			Error:  "no supported disk health collector for this device",
		}
	}
}

func collectNVMe(dev *smartlib.NVMeDevice) Result {
	sm, err := dev.ReadSMART()
	if err != nil {
		return Result{
			Source: block.DiskHealthSourceNVMe,
			Status: block.DiskHealthStatusValueUnknown,
			Error:  fmt.Sprintf("failed to read NVMe SMART data: %v", err),
		}
	}

	details := &block.DiskHealthNVMeDetails{
		CriticalWarning:             uint32(sm.CritWarning),
		PercentageUsed:              uint32(sm.PercentUsed),
		UnsafeShutdowns:             sm.UnsafeShutdowns.Val[0],
		MediaAndDataIntegrityErrors: sm.MediaErrors.Val[0],
	}

	status := ComputeNVMeStatus(details)

	return Result{
		Source:             block.DiskHealthSourceNVMe,
		Status:             status,
		TemperatureCelsius: int32(sm.Temperature) - 273,
		PowerOnHours:       sm.PowerOnHours.Val[0],
		PowerCycles:        sm.PowerCycles.Val[0],
		NVMe:               details,
	}
}

func collectATA(dev *smartlib.SataDevice) Result {
	page, err := dev.ReadSMARTData()
	if err != nil {
		return Result{
			Source: block.DiskHealthSourceATA,
			Status: block.DiskHealthStatusValueUnknown,
			Error:  fmt.Sprintf("failed to read ATA SMART data: %v", err),
		}
	}

	details := &block.DiskHealthATADetails{}

	for _, attr := range page.Attrs {
		switch attr.Id {
		case AttrReallocatedSectorCount:
			details.ReallocatedSectorCount = attr.ValueRaw
		case AttrCurrentPendingSectorCount:
			details.CurrentPendingSectorCount = attr.ValueRaw
		case AttrOfflineUncorrectableCount:
			details.OfflineUncorrectableCount = attr.ValueRaw
		case AttrReportedUncorrectableErrors:
			details.ReportedUncorrectableErrors = attr.ValueRaw
		case AttrWearLevelingCount:
			details.WearLevelingCount = attr.ValueRaw
		}
	}

	generic, err := dev.ReadGenericAttributes()
	if err != nil {
		return Result{
			Source: block.DiskHealthSourceATA,
			Status: ComputeATAStatus(details),
			ATA:    details,
			Error:  fmt.Sprintf("failed to read generic attributes: %v", err),
		}
	}

	status := ComputeATAStatus(details)

	return Result{
		Source:             block.DiskHealthSourceATA,
		Status:             status,
		TemperatureCelsius: int32(generic.Temperature),
		PowerOnHours:       generic.PowerOnHours,
		PowerCycles:        generic.PowerCycles,
		ATA:                details,
	}
}

// ComputeNVMeStatus derives the health status from NVMe details.
func ComputeNVMeStatus(d *block.DiskHealthNVMeDetails) block.DiskHealthStatusValue {
	if d.CriticalWarning != 0 || d.MediaAndDataIntegrityErrors > 0 {
		return block.DiskHealthStatusValueCritical
	}

	if d.PercentageUsed > 90 {
		return block.DiskHealthStatusValueWarning
	}

	return block.DiskHealthStatusValueHealthy
}

// ComputeATAStatus derives the health status from ATA details.
func ComputeATAStatus(d *block.DiskHealthATADetails) block.DiskHealthStatusValue {
	if d.OfflineUncorrectableCount > 0 {
		return block.DiskHealthStatusValueCritical
	}

	if d.CurrentPendingSectorCount > 0 {
		return block.DiskHealthStatusValueWarning
	}

	if d.ReallocatedSectorCount > 0 {
		return block.DiskHealthStatusValueWarning
	}

	return block.DiskHealthStatusValueHealthy
}
