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

// Names of the QoS class cgroups the kubelet creates directly under the kubepods
// cgroup. Guaranteed pods have no dedicated class cgroup: they live directly under
// kubepods, alongside these.
const (
	cgroupBesteffort = "besteffort"
	cgroupBurstable  = "burstable"
)

// qosClasses lists all QoS classes tracked in the OOM eval context.
var qosClasses = []runtime.QoSCgroupClass{
	runtime.QoSCgroupClassBesteffort,
	runtime.QoSCgroupClassBurstable,
	runtime.QoSCgroupClassGuaranteed,
	runtime.QoSCgroupClassPodruntime,
	runtime.QoSCgroupClassSystem,
}

// guaranteedCgroups lists the cgroup paths (relative to root) of Guaranteed QoS pods.
//
// The kubelet places Guaranteed pods directly under the kubepods cgroup, next to the
// besteffort/burstable class cgroups (which are excluded here), rather than in a
// dedicated "guaranteed" sub-cgroup.
func guaranteedCgroups(root string) []string {
	entries, err := os.ReadDir(filepath.Join(root, constants.CgroupKubepods))
	if err != nil {
		return nil
	}

	var paths []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if entry.Name() == cgroupBesteffort || entry.Name() == cgroupBurstable {
			continue
		}

		paths = append(paths, filepath.Join(constants.CgroupKubepods, entry.Name()))
	}

	return paths
}

// qosMap returns the per-class map stored under key in evalContext, creating it
// (pre-seeded with all QoS classes set to zero) if it doesn't exist yet.
//
// Pre-seeding keeps every QoS class present in the eval context even when no cgroup
// of that class currently exists (e.g. no Guaranteed pods scheduled).
func qosMap(evalContext map[string]any, key string) map[int]float64 {
	if v, ok := evalContext[key]; ok {
		return v.(map[int]float64)
	}

	m := make(map[int]float64, len(qosClasses))
	for _, class := range qosClasses {
		m[int(class)] = 0
	}

	evalContext[key] = m

	return m
}

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

	type subtree struct {
		path string
		qos  runtime.QoSCgroupClass
	}

	subtrees := []subtree{
		{"", -1},
		{constants.CgroupInit, runtime.QoSCgroupClassSystem},
		{constants.CgroupSystem, runtime.QoSCgroupClassSystem},
		{constants.CgroupPodRuntimeRoot, runtime.QoSCgroupClassPodruntime},
		{constants.CgroupKubepods + "/" + cgroupBesteffort, runtime.QoSCgroupClassBesteffort},
		{constants.CgroupKubepods + "/" + cgroupBurstable, runtime.QoSCgroupClassBurstable},
	}

	// Guaranteed pods live directly under the kubepods cgroup (there is no dedicated
	// "guaranteed" QoS cgroup), so aggregate the class metrics by summing over each
	// Guaranteed pod cgroup.
	for _, path := range guaranteedCgroups(cgroup) {
		subtrees = append(subtrees, subtree{path, runtime.QoSCgroupClassGuaranteed})
	}

	for _, subtree := range subtrees {
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
					qosMap(evalContext, "qos_memory_"+psiType+"_"+span)[int(subtree.qos)] += value
					qosMap(evalContext, "d_qos_memory_"+psiType+"_"+span)[int(subtree.qos)] += diff
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

			qosMap(evalContext, "qos_memory_"+parameter.name)[int(subtree.qos)] += value

			oldPath := subtree.path + "/" + "memory_" + parameter.name

			diff := 0.
			if oldValue, ok := oldValues[oldPath]; ok {
				diff = (value - oldValue) / sampleInterval.Seconds()
			}

			qosMap(evalContext, "d_qos_memory_"+parameter.name)[int(subtree.qos)] += diff
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
		skip  map[string]struct{}
	}{
		{constants.CgroupKubepods + "/" + cgroupBesteffort, runtime.QoSCgroupClassBesteffort, nil},
		{constants.CgroupKubepods + "/" + cgroupBurstable, runtime.QoSCgroupClassBurstable, nil},
		// Guaranteed pods live directly under the kubepods cgroup, alongside the
		// besteffort/burstable class cgroups which must be skipped here.
		{constants.CgroupKubepods, runtime.QoSCgroupClassGuaranteed, map[string]struct{}{
			cgroupBesteffort: {},
			cgroupBurstable:  {},
		}},
		{constants.CgroupPodRuntimeRoot, runtime.QoSCgroupClassPodruntime, nil},
		{constants.CgroupSystem, runtime.QoSCgroupClassSystem, nil},
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

			if _, skipped := cg.skip[leaf.Name()]; skipped {
				continue
			}

			cgroup, cgroupRank, ok := rankCgroupLeaf(logger, filepath.Join(root, cg.dir, leaf.Name()), cg.class, scoringExpr)
			if ok && cgroupRank > 0 {
				ranking[cgroup] = cgroupRank
			}
		}
	}

	return ranking
}

// rankCgroupLeaf reads the memory properties of a single cgroup and scores it.
//
// It returns the ranked cgroup, its score, and whether scoring succeeded.
func rankCgroupLeaf(logger *zap.Logger, leafDir string, class runtime.QoSCgroupClass, scoringExpr cel.Expression) (RankedCgroup, float64, bool) {
	node := cgroups.Node{}

	for _, prop := range []string{"memory.current", "memory.peak", "memory.max"} {
		if err := cgroups.ReadCgroupfsProperty(&node, leafDir, prop); err != nil {
			logger.Error(
				"cannot read property for cgroup",
				zap.String("dir", leafDir), zap.String("property", prop), zap.Error(err),
			)

			continue
		}
	}

	cgroup := RankedCgroup{
		Path:          leafDir,
		Class:         class,
		MemoryCurrent: node.MemoryCurrent,
		MemoryPeak:    node.MemoryPeak,
		MemoryMax:     node.MemoryMax,
	}

	cgroupRank, err := cgroup.CalculateScore(&scoringExpr)
	if err != nil {
		logger.Error(
			"cannot calculate score for cgroup",
			zap.String("dir", cgroup.Path), zap.Error(err),
		)

		return RankedCgroup{}, 0, false
	}

	return cgroup, cgroupRank, true
}

// SelectVictim picks the cgroup to OOM-kill. With strictClassOrdering it picks the
// lowest-importance QoS class with any eligible cgroup (score > 0), then the highest-scoring
// cgroup within that class; otherwise it picks the highest-scoring cgroup regardless of class.
func SelectVictim(ranking map[RankedCgroup]float64, strictClassOrdering bool) (RankedCgroup, float64, bool) {
	if !strictClassOrdering {
		return selectHighestScore(ranking)
	}

	const noClass = runtime.QoSCgroupClass(math.MaxInt)

	minClass := noClass

	for cgroup, score := range ranking {
		if score > 0 && cgroup.Class < minClass {
			minClass = cgroup.Class
		}
	}

	if minClass == noClass {
		return RankedCgroup{}, 0, false
	}

	var (
		maxScore = math.Inf(-1)
		victim   RankedCgroup
	)

	for cgroup, score := range ranking {
		if cgroup.Class == minClass && score > maxScore {
			maxScore = score
			victim = cgroup
		}
	}

	return victim, maxScore, true
}

// selectHighestScore picks the highest-scoring cgroup regardless of QoS class.
func selectHighestScore(ranking map[RankedCgroup]float64) (RankedCgroup, float64, bool) {
	if len(ranking) == 0 {
		return RankedCgroup{}, 0, false
	}

	var (
		maxScore = math.Inf(-1)
		victim   RankedCgroup
	)

	for cgroup, score := range ranking {
		if score > maxScore {
			maxScore = score
			victim = cgroup
		}
	}

	return victim, maxScore, true
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
