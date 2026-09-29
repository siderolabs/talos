// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/hardware"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//go:embed testdata/cpuscalingconfig.yaml
var expectedCPUScalingConfigDocument []byte

func TestCPUScalingConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := hardware.NewCPUScalingConfigV1Alpha1("performance-cores")
	cfg.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`cpu.core_type == "performance"`, celenv.CPUScalingLocator()))
	cfg.ScalingGovernor = "performance"
	cfg.ScalingEPP = "balance_performance"
	cfg.ScalingMinFrequencyKhz = 1200000
	cfg.ScalingMaxFrequencyKhz = 3300000

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedCPUScalingConfigDocument, marshaled)
}

func TestCPUScalingConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedCPUScalingConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	cfg, ok := docs[0].(*hardware.CPUScalingConfigV1Alpha1)
	require.True(t, ok)

	assert.Equal(t, "performance-cores", cfg.Name())
	assert.Equal(t, "performance", cfg.Governor())
	assert.Equal(t, "balance_performance", cfg.EnergyPerformancePreference())
	assert.EqualValues(t, 1200000, cfg.MinFrequencyKhz())
	assert.EqualValues(t, 3300000, cfg.MaxFrequencyKhz())
}

type validationMode struct{}

func (validationMode) String() string { return "" }

func (validationMode) RequiresInstall() bool { return false }

func (validationMode) InContainer() bool { return false }

func TestCPUScalingConfigValidate(t *testing.T) {
	t.Parallel()

	validSelector := cel.MustExpression(cel.ParseBooleanExpression(`true`, celenv.CPUScalingLocator()))

	for _, test := range []struct {
		name string
		cfg  func() *hardware.CPUScalingConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "valid",
			cfg: func() *hardware.CPUScalingConfigV1Alpha1 {
				c := hardware.NewCPUScalingConfigV1Alpha1("all")
				c.Selector.Match = validSelector
				c.ScalingGovernor = "performance"

				return c
			},
		},
		{
			name: "no name",
			cfg: func() *hardware.CPUScalingConfigV1Alpha1 {
				c := hardware.NewCPUScalingConfigV1Alpha1("")
				c.Selector.Match = validSelector
				c.ScalingGovernor = "performance"

				return c
			},
			expectedError: "name must be specified",
		},
		{
			name: "no selector",
			cfg: func() *hardware.CPUScalingConfigV1Alpha1 {
				c := hardware.NewCPUScalingConfigV1Alpha1("all")
				c.ScalingGovernor = "performance"

				return c
			},
			expectedError: "cpu scaling selector is required",
		},
		{
			name: "min above max",
			cfg: func() *hardware.CPUScalingConfigV1Alpha1 {
				c := hardware.NewCPUScalingConfigV1Alpha1("all")
				c.Selector.Match = validSelector
				c.ScalingMinFrequencyKhz = 3300000
				c.ScalingMaxFrequencyKhz = 1200000

				return c
			},
			expectedError: "minFrequencyKhz 3300000 is greater than maxFrequencyKhz 1200000",
		},
		{
			name: "governor with a slash",
			cfg: func() *hardware.CPUScalingConfigV1Alpha1 {
				c := hardware.NewCPUScalingConfigV1Alpha1("all")
				c.Selector.Match = validSelector
				c.ScalingGovernor = "../../etc/passwd"

				return c
			},
			expectedError: `governor "../../etc/passwd" contains invalid characters`,
		},
		{
			name: "sets nothing",
			cfg: func() *hardware.CPUScalingConfigV1Alpha1 {
				c := hardware.NewCPUScalingConfigV1Alpha1("all")
				c.Selector.Match = validSelector

				return c
			},
			expectedWarnings: []string{`CPUScalingConfig "all" sets nothing, so it has no effect`},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.expectedError)
			}
		})
	}
}

// TestSelectorFields pins down what a selector can read.
//
// Matching on a value the controller itself writes would form a feedback loop: the applied value
// would stop the selector matching, the attribute would be restored, and it would match again.
func TestSelectorFields(t *testing.T) {
	t.Parallel()

	for _, expr := range []string{
		`cpu.core_type == "performance"`,
		`cpu.cpu_info_max_frequency_khz > 3000000u`,
		`cpu.cpu_capacity < 1024u`,
		`cpu.driver == "intel_pstate"`,
		`"powersave" in cpu.available_governors`,
	} {
		_, err := cel.ParseBooleanExpression(expr, celenv.CPUScalingLocator())
		assert.NoError(t, err, "expression %q should compile", expr)
	}

	// the frequencies the controller sets are not reported, so a selector cannot chase them
	for _, expr := range []string{
		`cpu.min_frequency_khz > 0u`,
		`cpu.max_frequency_khz > 0u`,
		`cpu.no_such_field == "x"`,
	} {
		_, err := cel.ParseBooleanExpression(expr, celenv.CPUScalingLocator())
		assert.Error(t, err, "expression %q should not compile", expr)
	}

	// the governor is still reported, so it compiles; the controller blanks it before matching
	_, err := cel.ParseBooleanExpression(`cpu.governor == "performance"`, celenv.CPUScalingLocator())
	assert.NoError(t, err)
}

var _ validation.RuntimeMode = validationMode{}
