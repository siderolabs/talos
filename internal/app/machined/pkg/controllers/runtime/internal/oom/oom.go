// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package oom contains utilities for OOM handler.
package oom

import (
	"log"

	"github.com/google/cel-go/common/types"

	"github.com/siderolabs/talos/internal/pkg/cgroups"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// RankedCgroup contains information about a cgroup used for OOM handling.
type RankedCgroup struct {
	Class         runtime.QoSCgroupClass
	Path          string
	MemoryCurrent cgroups.Value
	MemoryPeak    cgroups.Value
	MemoryMax     cgroups.Value
}

func cgroupValueToFloat64(v cgroups.Value, evalContext map[string]any, key string) {
	if !v.IsSet || v.IsMax || v.Frac > 0 || v.Val < 0 {
		evalContext[key] = types.OptionalNone
	} else {
		evalContext[key] = types.OptionalOf(types.Uint(v.Val))
	}
}

// CalculateScore calculates the score of the cgroup for OOM handling.
//
// Higher score means the cgroup is more likely to be killed.
func (cgroup *RankedCgroup) CalculateScore(expr *cel.Expression) (float64, error) {
	evalContext := map[string]any{
		"class": int(cgroup.Class),
		"path":  cgroup.Path,
	}

	cgroupValueToFloat64(cgroup.MemoryCurrent, evalContext, "memory_current")
	cgroupValueToFloat64(cgroup.MemoryPeak, evalContext, "memory_peak")
	cgroupValueToFloat64(cgroup.MemoryMax, evalContext, "memory_max")

	log.Printf("Evaluating CalculateScore: evalContext = %v", evalContext)

	return expr.EvalDouble(celenv.OOMCgroupScoring(), evalContext)
}
