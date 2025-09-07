// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package oom_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/oom"
	"github.com/siderolabs/talos/internal/pkg/cgroups"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const expr1 = `memory_max.hasValue() ? 0.0 : 
			{Besteffort : 1.0, Guaranteed: 0.0, Burstable: 0.5}[class] * 
			   double(memory_current.orValue(0u)) / double(memory_peak.orValue(0u) - memory_current.orValue(0u))`

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
				MemoryMax:     cgroups.Value{IsSet: false, IsMax: true},
			},
			expect: 2.625,
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
