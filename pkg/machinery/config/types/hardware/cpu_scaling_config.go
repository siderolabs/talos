// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// CPUScalingConfigKind is a CPUScalingConfig config document kind.
const CPUScalingConfigKind = "CPUScalingConfig"

func init() {
	registry.Register(CPUScalingConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &CPUScalingConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.CPUScalingConfig = &CPUScalingConfigV1Alpha1{}
	_ config.NamedDocument    = &CPUScalingConfigV1Alpha1{}
	_ config.Validator        = &CPUScalingConfigV1Alpha1{}
)

// CPUScalingConfigV1Alpha1 configures Linux CPU frequency scaling.
//
//	examples:
//	  - value: exampleCPUScalingConfigV1Alpha1()
//	  - value: exampleCPUScalingConfigEfficiencyV1Alpha1()
//	alias: CPUScalingConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/CPUScalingConfig
type CPUScalingConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the config document.
	//
	//     It is used to tell apart several scaling policies, and is reported on the
	//     `CPUScalingSpec` resources the document produces.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Selector to match the cpufreq policies to configure.
	//
	//     If several documents match the same policy, the first one (in document order) wins.
	//   schemaRequired: true
	Selector CPUScalingSelector `yaml:"selector,omitempty"`
	//   description: |
	//     Scaling governor to set, e.g. `performance`, `powersave` or `schedutil`.
	//
	//     The governors a machine offers depend on its cpufreq driver, and are reported per policy
	//     in `availableGovernors` of the `CPUScalingStatus` resource.
	//   examples:
	//     - value: >
	//        "performance"
	ScalingGovernor string `yaml:"governor,omitempty"`
	//   description: |
	//     Energy performance preference to set, e.g. `performance`, `balance_performance`,
	//     `balance_power` or `power`.
	//
	//     Only some drivers implement this (HWP-enabled `intel_pstate`, `amd-pstate` in active
	//     mode); the values a machine offers are reported per policy in `availableEPPs` of the
	//     `CPUScalingStatus` resource.
	//   examples:
	//     - value: >
	//        "balance_performance"
	ScalingEPP string `yaml:"energyPerformancePreference,omitempty"`
	//   description: |
	//     Lower bound of the frequency window the governor may use, in kHz.
	//
	//     Left alone when not set.
	//   examples:
	//     - value: >
	//        1200000
	ScalingMinFrequencyKhz uint64 `yaml:"minFrequencyKhz,omitempty"`
	//   description: |
	//     Upper bound of the frequency window the governor may use, in kHz.
	//
	//     Left alone when not set.
	//   examples:
	//     - value: >
	//        3300000
	ScalingMaxFrequencyKhz uint64 `yaml:"maxFrequencyKhz,omitempty"`
}

// CPUScalingSelector selects the cpufreq policies to configure.
type CPUScalingSelector struct {
	//   description: |
	//     The Common Expression Language (CEL) expression to match the cpufreq policy.
	//
	//     The `cpu` variable is a cpufreq policy as reported by the `CPUScalingStatus` resource.
	//   schema:
	//     type: string
	//   examples:
	//    - value: >
	//        exampleCPUScalingSelectorAll()
	//      name: match every policy
	//    - value: >
	//        exampleCPUScalingSelectorPerformance()
	//      name: match the performance cores of a hybrid CPU
	//    - value: >
	//        exampleCPUScalingSelectorFast()
	//      name: match policies whose hardware tops out above 3 GHz
	//    - value: >
	//        exampleCPUScalingSelectorDriver()
	//      name: match policies by scaling driver
	Match cel.Expression `yaml:"match,omitempty"`
}

// NewCPUScalingConfigV1Alpha1 creates a new CPUScalingConfig config document.
func NewCPUScalingConfigV1Alpha1(name string) *CPUScalingConfigV1Alpha1 {
	return &CPUScalingConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       CPUScalingConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleCPUScalingConfigV1Alpha1() *CPUScalingConfigV1Alpha1 {
	cfg := NewCPUScalingConfigV1Alpha1("all-cores")
	cfg.Selector.Match = exampleCPUScalingSelectorAll()
	cfg.ScalingGovernor = "performance"

	return cfg
}

func exampleCPUScalingConfigEfficiencyV1Alpha1() *CPUScalingConfigV1Alpha1 {
	cfg := NewCPUScalingConfigV1Alpha1("efficiency-cores")
	cfg.Selector.Match = exampleCPUScalingSelectorEfficiency()
	cfg.ScalingGovernor = "powersave"

	return cfg
}

func exampleCPUScalingSelectorAll() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`true`, celenv.CPUScalingLocator()))
}

func exampleCPUScalingSelectorPerformance() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`cpu.core_type == "performance"`, celenv.CPUScalingLocator()))
}

func exampleCPUScalingSelectorEfficiency() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`cpu.core_type == "efficiency"`, celenv.CPUScalingLocator()))
}

func exampleCPUScalingSelectorFast() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`cpu.cpu_info_max_frequency_khz > 3000000u`, celenv.CPUScalingLocator()))
}

func exampleCPUScalingSelectorDriver() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(`cpu.driver == "intel_pstate"`, celenv.CPUScalingLocator()))
}

// Clone implements config.Document interface.
func (s *CPUScalingConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *CPUScalingConfigV1Alpha1) Name() string {
	return s.MetaName
}

// CPUScalingSelector implements config.CPUScalingConfig interface.
func (s *CPUScalingConfigV1Alpha1) CPUScalingSelector() cel.Expression {
	return s.Selector.Match
}

// Governor implements config.CPUScalingConfig interface.
func (s *CPUScalingConfigV1Alpha1) Governor() string {
	return s.ScalingGovernor
}

// EnergyPerformancePreference implements config.CPUScalingConfig interface.
func (s *CPUScalingConfigV1Alpha1) EnergyPerformancePreference() string {
	return s.ScalingEPP
}

// MinFrequencyKhz implements config.CPUScalingConfig interface.
func (s *CPUScalingConfigV1Alpha1) MinFrequencyKhz() uint64 {
	return s.ScalingMinFrequencyKhz
}

// MaxFrequencyKhz implements config.CPUScalingConfig interface.
func (s *CPUScalingConfigV1Alpha1) MaxFrequencyKhz() uint64 {
	return s.ScalingMaxFrequencyKhz
}

// Validate implements config.Validator interface.
//
// Which governors and preferences a machine accepts depends on its cpufreq driver, so only their
// shape can be checked here.
//
//nolint:gocyclo
func (s *CPUScalingConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	if s.Selector.Match.IsZero() {
		errs = errors.Join(errs, errors.New("cpu scaling selector is required"))
	} else if err := s.Selector.Match.ParseBool(celenv.CPUScalingLocator()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("cpu scaling selector is invalid: %w", err))
	}

	for _, v := range []struct {
		name  string
		value string
	}{
		{"governor", s.ScalingGovernor},
		{"energyPerformancePreference", s.ScalingEPP},
	} {
		if strings.ContainsFunc(v.value, func(r rune) bool { return r == '/' || r == '\n' || r == ' ' }) {
			errs = errors.Join(errs, fmt.Errorf("%s %q contains invalid characters", v.name, v.value))
		}
	}

	if s.ScalingMinFrequencyKhz != 0 && s.ScalingMaxFrequencyKhz != 0 && s.ScalingMinFrequencyKhz > s.ScalingMaxFrequencyKhz {
		errs = errors.Join(errs, fmt.Errorf("minFrequencyKhz %d is greater than maxFrequencyKhz %d", s.ScalingMinFrequencyKhz, s.ScalingMaxFrequencyKhz))
	}

	if s.ScalingGovernor == "" && s.ScalingEPP == "" && s.ScalingMinFrequencyKhz == 0 && s.ScalingMaxFrequencyKhz == 0 {
		return []string{fmt.Sprintf("CPUScalingConfig %q sets nothing, so it has no effect", s.MetaName)}, errs
	}

	return nil, errs
}
