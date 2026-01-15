// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package oom contains utilities for OOM handler.
package oom

import (
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"time"

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
func EvaluateTrigger(triggerExpr cel.Expression, evalContext map[string]any) (bool, error) {
	trigger, err := triggerExpr.EvalBool(celenv.OOMTrigger(), evalContext)
	if err != nil {
		return false, fmt.Errorf("cannot evaluate expression: %w", err)
	}

	return trigger, nil
}

// PopulatePsiToCtx populates the context with PSI data from a cgroup.
//
//nolint:gocyclo
func PopulatePsiToCtx(cgroup string, evalContext map[string]any, oldValues map[string]float64, sampleInterval time.Duration) error {
	if sampleInterval <= 0 {
		return fmt.Errorf("sample interval must be greater than zero")
	}

	for _, subtree := range []struct {
		path string
		qos  runtime.QoSCgroupClass
	}{
		{"", -1},
		{"init", runtime.QoSCgroupClassSystem},
		{"system", runtime.QoSCgroupClassSystem},
		{"podruntime", runtime.QoSCgroupClassPodruntime},
		{"kubepods/besteffort", runtime.QoSCgroupClassBesteffort},
		{"kubepods/burstable", runtime.QoSCgroupClassBurstable},
		{"kubepods/guaranteed", runtime.QoSCgroupClassGuaranteed},
	} {
		node, err := cgroups.GetCgroupProperty(filepath.Join(cgroup, subtree.path), "memory.pressure")

		for _, psiType := range []string{"some", "full"} {
			for _, span := range []string{"avg10", "avg60", "avg300", "total"} {
				value := 0.

				// Default non-existent cgroups to all-zero, e.g. during system boot
				if err == nil {
					value, err = extractPsiEntry(node, psiType, span)
					if err != nil {
						return err
					}
				}

				// calculate delta
				psiPath := subtree.path + "/" + "memory_" + psiType + "_" + span

				diff := 0.
				if oldValue, ok := oldValues[psiPath]; ok {
					diff = (value - oldValue) / sampleInterval.Seconds()
				}

				oldValues[psiPath] = value

				if subtree.qos == -1 {
					evalContext["d_memory_"+psiType+"_"+span] = diff
					evalContext["memory_"+psiType+"_"+span] = value
				} else {
					valuesMap, ok := evalContext["qos_memory_"+psiType+"_"+span]
					if !ok {
						valuesMap = map[int]float64{}
						evalContext["qos_memory_"+psiType+"_"+span] = valuesMap
					}

					valuesMap.(map[int]float64)[int(subtree.qos)] += value

					dValuesMap, ok := evalContext["d_qos_memory_"+psiType+"_"+span]
					if !ok {
						dValuesMap = map[int]float64{}
						evalContext["d_qos_memory_"+psiType+"_"+span] = dValuesMap
					}

					dValuesMap.(map[int]float64)[int(subtree.qos)] += diff
				}
			}
		}

		node = &cgroups.Node{}
		// Best effort, if any is not present it will return NaN
		cgroups.ReadCgroupfsProperty(node, filepath.Join(cgroup, subtree.path), "memory.current") //nolint:errcheck
		cgroups.ReadCgroupfsProperty(node, filepath.Join(cgroup, subtree.path), "memory.max")     //nolint:errcheck
		cgroups.ReadCgroupfsProperty(node, filepath.Join(cgroup, subtree.path), "memory.peak")    //nolint:errcheck

		if subtree.qos == -1 {
			continue
		}

		for _, parameter := range []struct {
			name  string
			value float64
		}{
			{"current", node.MemoryCurrent.Float64()},
			{"max", node.MemoryMax.Float64()},
			{"peak", node.MemoryPeak.Float64()},
		} {
			value := parameter.value
			// These values cannot be expressed in JSON
			if math.IsNaN(value) || math.IsInf(value, 0) {
				value = 0.0
			}

			valuesMap, ok := evalContext["qos_memory_"+parameter.name]
			if !ok {
				valuesMap = map[int]float64{}
				evalContext["qos_memory_"+parameter.name] = valuesMap
			}

			valuesMap.(map[int]float64)[int(subtree.qos)] += value

			oldPath := subtree.path + "/" + "memory_" + parameter.name

			diff := 0.
			if oldValue, ok := oldValues[oldPath]; ok {
				diff = (value - oldValue) / sampleInterval.Seconds()
			}

			dValuesMap, ok := evalContext["d_qos_memory_"+parameter.name]
			if !ok {
				dValuesMap = map[int]float64{}
				evalContext["d_qos_memory_"+parameter.name] = dValuesMap
			}

			dValuesMap.(map[int]float64)[int(subtree.qos)] += diff
		}
	}

	return nil
}

func extractPsiEntry(node *cgroups.Node, psiType string, span string) (float64, error) {
	spans, ok := node.MemoryPressure[psiType]
	if !ok {
		return 0, fmt.Errorf("cannot find memory pressure type: type: %s", psiType)
	}

	cgValue, ok := spans[span]
	if !ok {
		return 0, fmt.Errorf("cannot find memory pressure span: span: %s", span)
	}

	if !cgValue.IsSet || cgValue.IsMax {
		return 0, fmt.Errorf("PSI is not defined")
	}

	return cgValue.Float64(), nil
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
