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
		{
			name:      "empty",
			dir:       "./testdata/empty",
			expectErr: "cannot read memory pressure: error opening cgroupfs file open testdata/empty/memory.pressure: no such file or directory",
			expect:    map[string]any{},
		},
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
			},
		},
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
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := map[string]any{}

			err := oom.PopulatePsiToCtx(test.dir, ctx, make(map[string]float64), 0)

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
			expectErr:   "cannot read memory pressure: error opening cgroupfs file open testdata/empty/memory.pressure: no such file or directory",
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
			triggerExpr: triggerExpr1,
			expect:      false,
			expectErr:   "",
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
			}, 0)

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
