// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package oom contains utilities for OOM handler.
package oom

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/google/cel-go/common/types"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/cgroups"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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

func cgroupValueToOptionalUint(v cgroups.Value, evalContext map[string]any, key string) {
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

	cgroupValueToOptionalUint(cgroup.MemoryCurrent, evalContext, "memory_current")
	cgroupValueToOptionalUint(cgroup.MemoryPeak, evalContext, "memory_peak")
	cgroupValueToOptionalUint(cgroup.MemoryMax, evalContext, "memory_max")

	return expr.EvalDouble(celenv.OOMCgroupScoring(), evalContext)
}

// EvaluateTrigger is a method obtaining data and evaluating the trigger expression.
// When the result is true, designated OOM action is to be executed.
func EvaluateTrigger(triggerExpr cel.Expression, evalContext map[string]any, cgroup string) (bool, error) {
	err := PopulatePsiToCtx(cgroup, evalContext)
	if err != nil {
		return false, fmt.Errorf("cannot populate PSI context: %w", err)
	}

	trigger, err := triggerExpr.EvalBool(celenv.OOMTrigger(), evalContext)
	if err != nil {
		return false, fmt.Errorf("cannot evaluate expression: %w", err)
	}

	return trigger, nil
}

// PopulatePsiToCtx populates the context with PSI data from a cgroup.
func PopulatePsiToCtx(cgroup string, evalContext map[string]any) error {
	node, err := cgroups.GetCgroupProperty(cgroup, "memory.pressure")
	if err != nil {
		return fmt.Errorf("cannot read memory pressure: %w", err)
	}

	for _, psiType := range []string{"some", "full"} {
		for _, span := range []string{"avg10", "avg60", "avg300", "total"} {
			spans, ok := node.MemoryPressure[psiType]
			if !ok {
				return fmt.Errorf("cannot find memory pressure type: type: %s", psiType)
			}

			value, ok := spans[span]
			if !ok {
				return fmt.Errorf("cannot find memory pressure span: span: %s", span)
			}

			if !value.IsSet || value.IsMax {
				return fmt.Errorf("PSI is not defined")
			}

			evalContext["memory_"+psiType+"_"+span] = value.Float64()
		}
	}

	return nil
}

// RankCgroups ranks cgroups using a scoring expression and returns a map.
func RankCgroups(logger *zap.Logger, root string, scoringExpr cel.Expression) map[RankedCgroup]float64 {
	ranking := map[RankedCgroup]float64{}

	for _, cg := range []struct {
		dir   string
		class runtime.QoSCgroupClass
	}{
		{"kubepods/besteffort", runtime.QoSCgroupClassBesteffort},
		{"kubepods/burstable", runtime.QoSCgroupClassBurstable},
		{"kubepods/guaranteed", runtime.QoSCgroupClassGuaranteed},
		{constants.CgroupPodRuntimeRoot, runtime.QoSCgroupClassPodruntime},
		{constants.CgroupSystem, runtime.QoSCgroupClassSystem},
	} {
		entries, err := os.ReadDir(filepath.Join(root, cg.dir))
		if err != nil && !os.IsNotExist(err) {
			logger.Error("cannot list cgroup members", zap.String("dir", cg.dir), zap.Error(err))

			continue
		}

		for _, leaf := range entries {
			if !leaf.IsDir() {
				continue
			}

			leafDir := filepath.Join(root, cg.dir, leaf.Name())

			node := cgroups.Node{}

			for _, prop := range []string{"memory.current", "memory.peak", "memory.max"} {
				err := cgroups.ReadCgroupfsProperty(&node, leafDir, prop)
				if err != nil {
					logger.Error("cannot read property for cgroup",
						zap.String("dir", leafDir), zap.String("propery", prop), zap.Error(err),
					)

					continue
				}
			}

			cgroup := RankedCgroup{
				Path:          leafDir,
				Class:         cg.class,
				MemoryCurrent: node.MemoryCurrent,
				MemoryPeak:    node.MemoryPeak,
				MemoryMax:     node.MemoryMax,
			}

			ranking[cgroup], err = cgroup.CalculateScore(&scoringExpr)
			if err != nil {
				logger.Error("cannot calculate score for cgroup",
					zap.String("dir", cgroup.Path), zap.Error(err),
				)

				continue
			}
		}
	}

	return ranking
}

// ListCgroupProcs returns a list of process IDs for a given cgroup path.
func ListCgroupProcs(cgroupPath string) []int {
	processes := []int{}

	// Ignore errors, find as many processes as possible
	//nolint:errcheck
	filepath.WalkDir(cgroupPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if !d.IsDir() {
			return nil
		}

		node, err := cgroups.GetCgroupProperty(path, "cgroup.procs")
		if err != nil {
			return err
		}

		for _, p := range node.CgroupProcs {
			processes = append(processes, int(p.Val))
		}

		return nil
	})

	return processes
}
