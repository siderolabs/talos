// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package oom_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/oom"
	"github.com/siderolabs/talos/internal/pkg/cgroups"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const expr1 = constants.DefaultOOMCgroupRankingExpression

func TestCalculateScore(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		expr   string
		cgroup oom.RankedCgroup
		expect float64
	}{
		{
			name: "basic",
			expr: expr1,
			cgroup: oom.RankedCgroup{
				Class:         runtime.QoSCgroupClassBurstable,
				Path:          "/some/path",
				MemoryCurrent: cgroups.Value{Val: 42, IsSet: true},
				MemoryPeak:    cgroups.Value{Val: 50, IsSet: true},
				MemoryMax:     cgroups.Value{IsSet: true, IsMax: true},
			},
			expect: 21,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsedExpr, err := cel.ParseDoubleExpression(test.expr, celenv.OOMCgroupScoring())
			require.NoError(t, err)

			score, err := test.cgroup.CalculateScore(&parsedExpr)
			require.NoError(t, err)

			assert.Equal(t, test.expect, score)
		},
		)
	}
}

func TestRankCgroups(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		dir    string
		expr   string
		expect map[oom.RankedCgroup]float64
	}{
		{
			name: "basic",
			dir:  "./testdata/rank1",
			expr: expr1,
			expect: map[oom.RankedCgroup]float64{
				{
					Class:         runtime.QoSCgroupClassBesteffort,
					Path:          "testdata/rank1/kubepods/besteffort/pod123",
					MemoryCurrent: cgroups.Value{Val: 222593024, IsSet: true},
					MemoryPeak:    cgroups.Value{Val: 371011584, IsSet: true},
					MemoryMax:     cgroups.Value{IsMax: true, IsSet: true},
				}: 2.22593024e+08,
				{
					Class:         runtime.QoSCgroupClassBurstable,
					Path:          "testdata/rank1/kubepods/burstable/podABC",
					MemoryCurrent: cgroups.Value{Val: 42, IsSet: true},
					MemoryPeak:    cgroups.Value{Val: 50, IsSet: true},
					MemoryMax:     cgroups.Value{IsSet: true, IsMax: true},
				}: 21,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			logger := zap.New(nil)

			parsedExpr, err := cel.ParseDoubleExpression(test.expr, celenv.OOMCgroupScoring())
			require.NoError(t, err)

			result := oom.RankCgroups(logger, test.dir, parsedExpr)

			assert.Equal(t, test.expect, result)
		})
	}
}

func TestPopulatePsiToCtx(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		dir       string
		expectErr string
		expect    map[string]any
	}{
		//nolint:dupl
		{
			name:      "empty",
			dir:       "./testdata/empty",
			expectErr: "",
			expect: map[string]any{
				"memory_full_avg10":    0.0,
				"memory_full_avg300":   0.0,
				"memory_full_avg60":    0.0,
				"memory_full_total":    0.0,
				"memory_some_avg10":    0.0,
				"memory_some_avg300":   0.0,
				"memory_some_avg60":    0.0,
				"memory_some_total":    0.0,
				"d_memory_full_avg10":  0.0,
				"d_memory_full_avg300": 0.0,
				"d_memory_full_avg60":  0.0,
				"d_memory_full_total":  0.0,
				"d_memory_some_avg10":  0.0,
				"d_memory_some_avg300": 0.0,
				"d_memory_some_avg60":  0.0,
				"d_memory_some_total":  0.0,

				"qos_memory_some_avg10": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"qos_memory_some_avg60": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"qos_memory_some_avg300": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"qos_memory_some_total": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"qos_memory_full_avg10": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"qos_memory_full_avg60": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"qos_memory_full_avg300": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"qos_memory_full_total": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_some_avg10": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_some_avg60": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_some_avg300": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_some_total": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_full_avg10": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_full_avg60": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_full_avg300": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_full_total": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},

				"qos_memory_current": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_current": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},

				"qos_memory_peak": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_peak": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},

				"qos_memory_max": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_max": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
			},
		},
		//nolint:dupl
		{
			name:      "false",
			dir:       "./testdata/trigger-false",
			expectErr: "",
			expect: map[string]any{
				"memory_full_avg10":    2.4,
				"memory_full_avg300":   1.71,
				"memory_full_avg60":    5.16,
				"memory_full_total":    1.0654831e+07,
				"memory_some_avg10":    2.82,
				"memory_some_avg300":   1.97,
				"memory_some_avg60":    5.95,
				"memory_some_total":    1.217234e+07,
				"d_memory_full_avg10":  0.0,
				"d_memory_full_avg300": 0.0,
				"d_memory_full_avg60":  0.0,
				"d_memory_full_total":  0.0,
				"d_memory_some_avg10":  0.0,
				"d_memory_some_avg300": 0.0,
				"d_memory_some_avg60":  0.0,
				"d_memory_some_total":  0.0,

				"qos_memory_some_avg10": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 2.82,
					int(runtime.QoSCgroupClassSystem):     5.64,
				},
				"qos_memory_some_avg60": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 5.95,
					int(runtime.QoSCgroupClassSystem):     11.9,
				},
				"qos_memory_some_avg300": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 1.97,
					int(runtime.QoSCgroupClassSystem):     3.94,
				},
				"qos_memory_some_total": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 1.217234e+07,
					int(runtime.QoSCgroupClassSystem):     2.434468e+07,
				},
				"qos_memory_full_avg10": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 2.4,
					int(runtime.QoSCgroupClassSystem):     4.8,
				},
				"qos_memory_full_avg60": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 5.16,
					int(runtime.QoSCgroupClassSystem):     10.32,
				},
				"qos_memory_full_avg300": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 1.71,
					int(runtime.QoSCgroupClassSystem):     3.42,
				},
				"qos_memory_full_total": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 1.0654831e+07,
					int(runtime.QoSCgroupClassSystem):     1.0654937e+07,
				},
				"d_qos_memory_some_avg10": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_some_avg60": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_some_avg300": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_some_total": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_full_avg10": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_full_avg60": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_full_avg300": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_full_total": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},

				"qos_memory_current": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_current": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},

				"qos_memory_peak": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_peak": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},

				"qos_memory_max": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_max": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
			},
		},
		// //nolint:dupl
		{
			name:      "true",
			dir:       "./testdata/trigger-true",
			expectErr: "",
			expect: map[string]any{
				"memory_full_avg10":    14.54,
				"memory_full_avg60":    6.97,
				"memory_full_avg300":   1.82,
				"memory_full_total":    1.0654831e+07,
				"memory_some_avg10":    17.06,
				"memory_some_avg60":    8.04,
				"memory_some_avg300":   2.1,
				"memory_some_total":    1.217234e+07,
				"d_memory_full_avg10":  0.0,
				"d_memory_full_avg300": 0.0,
				"d_memory_full_avg60":  0.0,
				"d_memory_full_total":  0.0,
				"d_memory_some_avg10":  0.0,
				"d_memory_some_avg300": 0.0,
				"d_memory_some_avg60":  0.0,
				"d_memory_some_total":  0.0,

				"qos_memory_some_avg10": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 17.06,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 17.06,
					int(runtime.QoSCgroupClassSystem):     34.12,
				},
				"qos_memory_some_avg60": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 8.04,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 8.04,
					int(runtime.QoSCgroupClassSystem):     16.08,
				},
				"qos_memory_some_avg300": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 2.1,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 2.1,
					int(runtime.QoSCgroupClassSystem):     4.2,
				},
				"qos_memory_some_total": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 1.217234e+07,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 1.217234e+07,
					int(runtime.QoSCgroupClassSystem):     2.434468e+07,
				},
				"qos_memory_full_avg10": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 14.54,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 14.54,
					int(runtime.QoSCgroupClassSystem):     29.08,
				},
				"qos_memory_full_avg60": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 6.97,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 6.97,
					int(runtime.QoSCgroupClassSystem):     13.94,
				},
				"qos_memory_full_avg300": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 1.82,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 1.82,
					int(runtime.QoSCgroupClassSystem):     3.64,
				},
				"qos_memory_full_total": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 1.0654831e+07,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 1.0654831e+07,
					int(runtime.QoSCgroupClassSystem):     2.1309662e+07,
				},
				"d_qos_memory_some_avg10": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_some_avg60": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_some_avg300": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_some_total": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_full_avg10": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_full_avg60": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_full_avg300": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_full_total": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},

				"qos_memory_current": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_current": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},

				"qos_memory_peak": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_peak": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},

				"qos_memory_max": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
				"d_qos_memory_max": map[int]float64{
					int(runtime.QoSCgroupClassBesteffort): 0.0,
					int(runtime.QoSCgroupClassBurstable):  0.0,
					int(runtime.QoSCgroupClassGuaranteed): 0.0,
					int(runtime.QoSCgroupClassPodruntime): 0.0,
					int(runtime.QoSCgroupClassSystem):     0.0,
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := map[string]any{}

			err := oom.PopulatePsiToCtx(test.dir, ctx, make(map[string]float64), 500*time.Millisecond)

			if test.expectErr == "" {
				require.NoError(t, err)
				assert.Equal(t, test.expect, ctx)
			} else {
				assert.ErrorContains(t, err, test.expectErr)
			}
		})
	}
}

func TestEvaluateTrigger(t *testing.T) {
	t.Parallel()

	triggerExpr1 := cel.MustExpression(cel.ParseBooleanExpression(
		constants.DefaultOOMTriggerExpression,
		celenv.OOMTrigger(),
	))

	for _, test := range []struct {
		name        string
		dir         string
		ctx         map[string]any
		triggerExpr cel.Expression
		expect      bool
		expectErr   string
	}{
		{
			name: "empty",
			dir:  "./testdata/empty",
			ctx: map[string]any{
				"time_since_trigger": 3 * time.Second,
			},
			triggerExpr: triggerExpr1,
			expect:      false,
			expectErr:   "",
		},
		{
			name: "cgroup-false",
			dir:  "./testdata/trigger-false",
			ctx: map[string]any{
				"time_since_trigger": 3 * time.Second,
			},
			triggerExpr: triggerExpr1,
			expect:      false,
			expectErr:   "",
		},
		{
			name: "cgroup-true-cool",
			dir:  "./testdata/trigger-true",
			ctx: map[string]any{
				"time_since_trigger": 3 * time.Second,
			},
			triggerExpr: triggerExpr1,
			expect:      true,
			expectErr:   "",
		},
		{
			name: "cgroup-true-hot",
			dir:  "./testdata/trigger-true",
			ctx: map[string]any{
				"time_since_trigger": 300 * time.Millisecond,
			},
			triggerExpr: cel.MustExpression(cel.ParseBooleanExpression(
				`memory_full_avg10 > 12.0 && time_since_trigger > duration("500ms")`,
				celenv.OOMTrigger(),
			)),
			expect:    false,
			expectErr: "",
		},
		{
			name: "cgroup-true-hot-overridden",
			dir:  "./testdata/trigger-true",
			ctx: map[string]any{
				"time_since_trigger": 300 * time.Millisecond,
			},
			triggerExpr: cel.MustExpression(cel.ParseBooleanExpression(
				`memory_full_avg10 > 12.0 && time_since_trigger > duration("250ms")`,
				celenv.OOMTrigger(),
			)),
			expect:    true,
			expectErr: "",
		},
		{
			name: "test multiply_qos",
			ctx:  map[string]any{},
			dir:  "./testdata/trigger-true",
			triggerExpr: cel.MustExpression(cel.ParseBooleanExpression(
				// 5 * 1 + 2 * -1 + 0 * 3 == 3
				`multiply_qos_vectors({Besteffort: 5.0, Burstable: 2.0, Guaranteed: 0.0, System: 1.0}, {Besteffort: 1.0, Burstable: -1.0, Guaranteed: 3.0}) == 3.0`,
				celenv.OOMTrigger(),
			)),
			expect: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := oom.PopulatePsiToCtx(test.dir, test.ctx, map[string]float64{
				"memory_full_avg10":  0,
				"memory_full_avg300": 0,
				"memory_full_avg60":  0,
				"memory_full_total":  0,
				"memory_some_avg10":  0,
				"memory_some_avg300": 0,
				"memory_some_avg60":  0,
				"memory_some_total":  0,

				"init/memory_full_total": 0,
			}, 500*time.Millisecond)

			if test.expectErr == "" {
				require.NoError(t, err)

				trigger, err := oom.EvaluateTrigger(test.triggerExpr, test.ctx)

				assert.Equal(t, test.expect, trigger)

				require.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.expectErr)
			}
		})
	}
}

func TestListCgroupProcs(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		dir    string
		expect []int
	}{
		{
			name:   "pod123",
			dir:    "testdata/rank1/kubepods/besteffort/pod123",
			expect: []int{1},
		},
		{
			name:   "podABC",
			dir:    "testdata/rank1/kubepods/burstable/podABC",
			expect: []int{132, 142536},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expect, oom.ListCgroupProcs(test.dir))
		})
	}
}
