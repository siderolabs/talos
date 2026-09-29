// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"testing"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hardwarectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	configpkg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hardwarecfg "github.com/siderolabs/talos/pkg/machinery/config/types/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

type CPUScalingConfigSuite struct {
	ctest.DefaultSuite
}

// addPolicy publishes a CPUScalingStatus, standing in for what CPUScalingStatusController discovers.
func (suite *CPUScalingConfigSuite) addPolicy(id string, spec hardware.CPUScalingStatusSpec) {
	res := hardware.NewCPUScalingStatus(id)
	*res.TypedSpec() = spec

	suite.Create(res)
}

func (suite *CPUScalingConfigSuite) setConfig(docs ...*hardwarecfg.CPUScalingConfigV1Alpha1) {
	suite.Create(config.NewMachineConfigWithID(newContainer(suite.T(), docs...), config.ActiveID))
}

// newContainer wraps CPUScalingConfig documents into a machine config container.
func newContainer(t *testing.T, docs ...*hardwarecfg.CPUScalingConfigV1Alpha1) *container.Container {
	t.Helper()

	cfg, err := container.New(xslices.Map(docs, func(doc *hardwarecfg.CPUScalingConfigV1Alpha1) configpkg.Document {
		return doc
	})...)
	require.NoError(t, err)

	return cfg
}

func scalingDoc(name, match, governor string) *hardwarecfg.CPUScalingConfigV1Alpha1 {
	doc := hardwarecfg.NewCPUScalingConfigV1Alpha1(name)
	doc.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(match, celenv.CPUScalingLocator()))
	doc.ScalingGovernor = governor

	return doc
}

// TestSelectorsSplitHeterogeneousCPU is the point of the whole design: two documents each claim the
// cores they describe, without either naming a CPU index or knowing how many cores there are.
func (suite *CPUScalingConfigSuite) TestSelectorsSplitHeterogeneousCPU() {
	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.CPUScalingConfigController{}))

	suite.addPolicy("policy0", hardware.CPUScalingStatusSpec{
		Driver: "intel_pstate", AffectedCPUs: []uint32{0}, CoreType: hardware.CoreTypePerformance,
	})
	suite.addPolicy("policy1", hardware.CPUScalingStatusSpec{
		Driver: "intel_pstate", AffectedCPUs: []uint32{1}, CoreType: hardware.CoreTypeEfficiency,
	})

	suite.setConfig(
		scalingDoc("a-performance", `cpu.core_type == "performance"`, "performance"),
		scalingDoc("b-efficiency", `cpu.core_type == "efficiency"`, "powersave"),
	)

	ctest.AssertResource(suite, "policy0", func(r *hardware.CPUScalingSpec, asrt *assert.Assertions) {
		asrt.Equal("performance", r.TypedSpec().Governor)
		asrt.Equal("a-performance", r.TypedSpec().ConfigName)
	})

	ctest.AssertResource(suite, "policy1", func(r *hardware.CPUScalingSpec, asrt *assert.Assertions) {
		asrt.Equal("powersave", r.TypedSpec().Governor)
		asrt.Equal("b-efficiency", r.TypedSpec().ConfigName)
	})
}

func TestCPUScalingConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CPUScalingConfigSuite{
		Timeout: 10 * time.Second,
	})
}

type CPUScalingConfigOverlapSuite struct {
	ctest.DefaultSuite
}

// TestFirstMatchWins covers two selectors claiming the same policy: resolution is by document name,
// not by the order documents happen to sit in after merging.
func (suite *CPUScalingConfigOverlapSuite) TestFirstMatchWins() {
	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.CPUScalingConfigController{}))

	res := hardware.NewCPUScalingStatus("policy0")
	*res.TypedSpec() = hardware.CPUScalingStatusSpec{Driver: "acpi-cpufreq", AffectedCPUs: []uint32{0}}
	suite.Create(res)

	suite.Create(config.NewMachineConfigWithID(newContainer(suite.T(),
		scalingDoc("zzz-last", `true`, "powersave"),
		scalingDoc("aaa-first", `true`, "performance"),
	), config.ActiveID))

	ctest.AssertResource(suite, "policy0", func(r *hardware.CPUScalingSpec, asrt *assert.Assertions) {
		asrt.Equal("performance", r.TypedSpec().Governor)
		asrt.Equal("aaa-first", r.TypedSpec().ConfigName)
	})
}

// TestUnmatchedPolicyGetsNoSpec covers a policy no selector claims: it is left alone entirely.
func (suite *CPUScalingConfigOverlapSuite) TestUnmatchedPolicyGetsNoSpec() {
	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.CPUScalingConfigController{}))

	for id, coreType := range map[string]string{
		"policy0": hardware.CoreTypePerformance,
		"policy1": "",
	} {
		res := hardware.NewCPUScalingStatus(id)
		*res.TypedSpec() = hardware.CPUScalingStatusSpec{Driver: "intel_pstate", CoreType: coreType}

		suite.Create(res)
	}

	suite.Create(config.NewMachineConfigWithID(newContainer(suite.T(),
		scalingDoc("perf-only", `cpu.core_type == "performance"`, "performance"),
	), config.ActiveID))

	ctest.AssertResource(suite, "policy0", func(r *hardware.CPUScalingSpec, asrt *assert.Assertions) {
		asrt.Equal("performance", r.TypedSpec().Governor)
	})

	ctest.AssertNoResource[*hardware.CPUScalingSpec](suite, "policy1")
}

// TestSelectorCannotSeeAppliedValues guards the feedback loop: a selector reading a field the
// controller itself writes must match nothing, or config -> spec -> status -> config would oscillate.
func (suite *CPUScalingConfigOverlapSuite) TestSelectorCannotSeeAppliedValues() {
	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.CPUScalingConfigController{}))

	res := hardware.NewCPUScalingStatus("policy0")
	*res.TypedSpec() = hardware.CPUScalingStatusSpec{Driver: "intel_pstate", Governor: "schedutil"}
	suite.Create(res)

	suite.Create(config.NewMachineConfigWithID(newContainer(suite.T(),
		scalingDoc("chase", `cpu.governor == "schedutil"`, "performance"),
	), config.ActiveID))

	ctest.AssertNoResource[*hardware.CPUScalingSpec](suite, "policy0")
}

func TestCPUScalingConfigOverlapSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CPUScalingConfigOverlapSuite{
		Timeout: 10 * time.Second,
	})
}
