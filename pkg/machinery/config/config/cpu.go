// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "github.com/siderolabs/talos/pkg/machinery/cel"

// CPUScalingConfig defines the interface to access CPU frequency scaling configuration.
//
// Settings apply to the cpufreq policies matched by the selector; an unset value leaves the
// corresponding attribute alone rather than resetting it.
type CPUScalingConfig interface {
	NamedDocument

	// CPUScalingSelector matches the cpufreq policies to configure.
	CPUScalingSelector() cel.Expression
	// Governor is the scaling governor to set, empty to leave alone.
	Governor() string
	// EnergyPerformancePreference is the energy performance preference to set, empty to leave alone.
	EnergyPerformancePreference() string
	// MinFrequencyKhz is the lower bound of the frequency window to set in kHz, zero to leave alone.
	MinFrequencyKhz() uint64
	// MaxFrequencyKhz is the upper bound of the frequency window to set in kHz, zero to leave alone.
	MaxFrequencyKhz() uint64
}
