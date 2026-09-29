// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package smartprobe implements probing a disk for SMART data using the smart.go library.
package smartprobe

import (
	"slices"

	smart "github.com/anatol/smart.go"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// sataDevice is the subset of *smart.SataDevice behavior Prober
// depends on, narrowed to an interface so it can be faked in tests.
type sataDevice interface {
	smart.Device
	CheckPowerMode() (byte, error)
	ReadSMARTData() (*smart.AtaSmartPage, error)
	ReadSMARTThresholds() (*smart.AtaSmartThresholdsPage, error)
}

// nvmeDevice is the subset of *smart.NVMeDevice behavior Prober
// depends on, narrowed to an interface so it can be faked in tests.
type nvmeDevice interface {
	smart.Device
	ReadSMART() (*smart.NvmeSMARTLog, error)
}

// Prober is the default SMART prober backed by the smart.go library.
type Prober struct {
	// Open opens the device at devPath; defaults to smart.Open, overridable in tests.
	Open func(devPath string) (smart.Device, error)
}

// Probe reads SMART data for the disk at devPath.
//
// When rotational is true, the disk power mode is checked first and, if the disk
// is in standby, standby is returned true and the SMART data is not read (so the
// disk is not spun up).
func (p Prober) Probe(devPath string, rotational bool) (block.SMARTStatusSpec, bool, error) {
	open := p.Open
	if open == nil {
		open = smart.Open
	}

	dev, err := open(devPath)
	if err != nil {
		return block.SMARTStatusSpec{}, false, err
	}

	defer dev.Close() //nolint:errcheck

	spec := block.SMARTStatusSpec{
		DevPath: devPath,
		DevType: dev.Type(),
		Healthy: true,
	}

	switch d := dev.(type) {
	case sataDevice:
		// avoid spinning up a standby disk just to read SMART.
		if rotational {
			if mode, err := d.CheckPowerMode(); err == nil {
				spec.PowerState = powerModeString(mode)

				if mode == smart.PowerModeStandby {
					return spec, true, nil
				}
			}
		}

		if err := fillSATA(&spec, d); err != nil {
			return block.SMARTStatusSpec{}, false, err
		}
	case nvmeDevice:
		if err := fillNVMe(&spec, d); err != nil {
			return block.SMARTStatusSpec{}, false, err
		}
	default:
		fillGeneric(&spec, dev)
	}

	return spec, false, nil
}

func fillGeneric(spec *block.SMARTStatusSpec, dev smart.Device) {
	attrs, err := dev.ReadGenericAttributes()
	if err != nil {
		return
	}

	spec.Temperature = uint32(attrs.Temperature)
	spec.PowerOnHours = attrs.PowerOnHours
	spec.PowerCycles = attrs.PowerCycles
}

func fillNVMe(spec *block.SMARTStatusSpec, d nvmeDevice) error {
	sm, err := d.ReadSMART()
	if err != nil {
		return err
	}

	spec.CriticalWarning = uint32(sm.CritWarning)
	spec.Healthy = sm.CritWarning == 0
	spec.AvailableSpare = uint32(sm.AvailSpare)
	spec.PercentUsed = uint32(sm.PercentUsed)
	spec.PowerOnHours = sm.PowerOnHours.Val[0]
	spec.PowerCycles = sm.PowerCycles.Val[0]
	spec.MediaErrors = sm.MediaErrors.Val[0]

	// Composite temperature is reported in Kelvin.
	if sm.Temperature >= 273 {
		spec.Temperature = uint32(sm.Temperature) - 273
	}

	return nil
}

func fillSATA(spec *block.SMARTStatusSpec, d sataDevice) error {
	page, err := d.ReadSMARTData()
	if err != nil {
		return err
	}

	// thresholds are best-effort; without them we can't compute per-attribute failure.
	thresholds, _ := d.ReadSMARTThresholds() //nolint:errcheck

	healthy := true

	for id, attr := range page.Attrs {
		sa := block.SMARTAttribute{
			ID:       uint32(attr.Id),
			Name:     attr.Name,
			Current:  uint32(attr.Current),
			Worst:    uint32(attr.Worst),
			RawValue: attr.ValueRaw,
		}

		if thresholds != nil {
			if th, ok := thresholds.Thresholds[id]; ok {
				sa.Threshold = uint32(th)

				// an attribute is failing when its normalized current value drops to or
				// below the threshold; a failing pre-failure attribute marks the disk unhealthy.
				if th > 0 && attr.Current <= th {
					sa.Failing = true

					if attr.Flags&smart.AtaAttributeFlagPrefailure != 0 {
						healthy = false
					}
				}
			}
		}

		spec.Attributes = append(spec.Attributes, sa)
	}

	// sort attributes by ID for a stable resource representation (map order is random).
	slices.SortFunc(spec.Attributes, func(a, b block.SMARTAttribute) int {
		return int(a.ID) - int(b.ID)
	})

	spec.Healthy = healthy

	fillGeneric(spec, d)

	return nil
}

func powerModeString(mode byte) string {
	switch mode {
	case smart.PowerModeStandby:
		return "standby"
	case smart.PowerModeIdle:
		return "idle"
	default:
		return "active"
	}
}
