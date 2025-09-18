// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/oom"
	"github.com/siderolabs/talos/internal/pkg/cgroups"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// OOMController is a controller that monitors memory PSI and handles near-OOM situations.
type OOMController struct {
	CgroupRoot      string
	ActionTriggered time.Time
}

// Name implements controller.Controller interface.
func (ctrl *OOMController) Name() string {
	return "runtime.OOMController"
}

// Inputs implements controller.Controller interface.
func (ctrl *OOMController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *OOMController) Outputs() []controller.Output {
	return nil
}

var defaultTriggerExpr = sync.OnceValue(func() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(
		`memory_full_avg10 > 12.0 && time_since_trigger > duration("500ms")`,
		celenv.OOMTrigger(),
	))
})

// Sort processes by the following hierarchy:
// First, sort by high-level group:
//     kubepods (workloads)
//     podruntime (CRI, kubelet, etcd)
//     runtime (core containerd, system services)
//     init
// Second, inside kubepods we have QoS groups:
//     first priority: BestEffort
//     second: Burstable
//     last: Guaranteed
// Third, look into other attributes, e.g. OOM score.
// Fourth, look into memory max - memory current (if memory max is set).
//
// Sort to make the most prioritized to OOM-kill cgroup to the first place

var defaultScoringExpr = sync.OnceValue(func() cel.Expression {
	return cel.MustExpression(cel.ParseDoubleExpression(
		`memory_max.hasValue() ? 0.0 :
			{Besteffort : 1.0, Guaranteed: 0.0, Burstable: 0.5}[class] *
			   double(memory_current.orValue(0u)) / double(memory_peak.orValue(0u) - memory_current.orValue(0u))`,
		celenv.OOMCgroupScoring(),
	))
})

const defaultSampleInterval = 500 * time.Millisecond

// Run implements controller.Controller interface.
func (ctrl *OOMController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	triggerExpr := defaultTriggerExpr()
	scoringExpr := defaultScoringExpr()
	sampleInterval := defaultSampleInterval

	ticker := time.NewTicker(sampleInterval)
	tickerC := ticker.C

	if ctrl.CgroupRoot == "" {
		ctrl.CgroupRoot = constants.CgroupMountPath
	}

	for {
		// the controller runs a single time
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("cannot get active machine config: %w", err)
			}

			var newInterval time.Duration

			triggerExpr, scoringExpr, newInterval = ctrl.getConfig(cfg)

			if sampleInterval != newInterval {
				ticker.Reset(newInterval)
				sampleInterval = newInterval
			}
		case <-tickerC:
		}

		if ctrl.EvaluateTrigger(logger, triggerExpr) {
			ctrl.ActionTriggered = time.Now()
			ctrl.OomAction(logger, scoringExpr)
		}
	}
}

// EvaluateTrigger is a method obtaining data and evaluating the trigger expression.
// When the result is true, designated OOM action is to be executed.
func (ctrl *OOMController) EvaluateTrigger(logger *zap.Logger, triggerExpr cel.Expression) bool {
	evalContext := map[string]any{
		"time_since_trigger": time.Since(ctrl.ActionTriggered),
	}

	err := ctrl.populatePsiToCtx(evalContext)
	if err != nil {
		logger.Error("!!! ctxFailed ctxFailed ctxFailed", zap.Error(err))

		return false
	}

	trigger, err := triggerExpr.EvalBool(celenv.OOMTrigger(), evalContext)
	if err != nil {
		logger.Error("cannot evaluate trigger condition:", zap.Error(err))

		return false
	}

	// FIXME: remove hot-path logging
	logger.Info("Evaluated OOMTrigger", zap.Any("evalContext", evalContext), zap.Bool("result", trigger))

	return trigger
}

func (ctrl *OOMController) populatePsiToCtx(evalContext map[string]any) error {
	node, err := cgroups.GetCgroupProperty(constants.CgroupMountPath, "memory.pressure")
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

			val, err := strconv.ParseFloat(value.String(), 64)
			if err != nil {
				return fmt.Errorf("cannot parse memory pressure: span: %s, err %w", span, err)
			}

			evalContext["memory_"+psiType+"_"+span] = val
		}
	}

	return nil
}

func (*OOMController) getConfig(cfg *config.MachineConfig) (cel.Expression, cel.Expression, time.Duration) {
	triggerExpr := defaultTriggerExpr()

	if cfg != nil {
		if oomCfg := cfg.Config().OOMConfig(); oomCfg != nil {
			if expr, ok := oomCfg.TriggerExpression().Get(); ok {
				triggerExpr = expr
			}
		}
	}

	scoringExpr := defaultScoringExpr()

	if cfg != nil {
		if oomCfg := cfg.Config().OOMConfig(); oomCfg != nil {
			if expr, ok := oomCfg.CgroupRankingExpression().Get(); ok {
				scoringExpr = expr
			}
		}
	}

	newInterval := defaultSampleInterval

	if cfg != nil {
		if oomCfg := cfg.Config().OOMConfig(); oomCfg != nil {
			if interval, ok := oomCfg.SampleInterval().Get(); ok {
				newInterval = interval
			}
		}
	}

	return triggerExpr, scoringExpr, newInterval
}

// OomAction handles out of memory conditions by selecting and killing cgroups based on memory usage data.
func (ctrl *OOMController) OomAction(logger *zap.Logger, scoringExpr cel.Expression) {
	logger.Info("OOM controller triggered")

	ranking := ctrl.rankCgroups(logger, scoringExpr)

	if len(ranking) == 0 {
		return
	}

	var (
		maxScore     = math.Inf(-1)
		cgroupToKill oom.RankedCgroup
	)

	for cgroup, score := range ranking {
		if score > maxScore {
			maxScore = score
			cgroupToKill = cgroup
		}
	}

	err := ctrl.reapCg(logger, cgroupToKill.Path)
	if err != nil {
		logger.Error("cannot reap cgroup", zap.String("cgroup", cgroupToKill.Path), zap.Error(err))
	}
}

func (ctrl *OOMController) rankCgroups(logger *zap.Logger, scoringExpr cel.Expression) map[oom.RankedCgroup]float64 {
	ranking := map[oom.RankedCgroup]float64{}

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
		entries, err := os.ReadDir(filepath.Join(constants.CgroupMountPath, cg.dir))
		if err != nil {
			logger.Error("cannot list cgroup members", zap.String("dir", cg.dir), zap.Error(err))

			continue
		}

		for _, leaf := range entries {
			if !leaf.IsDir() {
				continue
			}

			leafDir := filepath.Join(constants.CgroupMountPath, cg.dir, leaf.Name())

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

			cgroup := oom.RankedCgroup{
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

func (ctrl *OOMController) reapCg(logger *zap.Logger, cgroupPath string) error {
	logger.Info("Sending SIGKILL to cgroup", zap.String("cgroup", cgroupPath))

	processes := []int{}
	// Ignore errors, find as many processes as possible
	//nolint:errcheck
	filepath.WalkDir(cgroupPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
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
	logger.Info("victim processes:", zap.Any("processes", processes))

	// Open pidfd's of all the processes in cgroup to accelerate kernel
	// garbage-collecting those processes via mrelease.
	pidfds := []int{}

	for _, pid := range processes {
		pidfd, err := unix.PidfdOpen(pid, 0)
		if err != nil {
			logger.Error("failed to open pidfd", zap.Int("pid", pid), zap.Error(err))

			continue
		}
		defer unix.Close(pidfd) //nolint:errcheck

		pidfds = append(pidfds, pidfd)
	}

	err := os.WriteFile(filepath.Join(cgroupPath, "cgroup.kill"), []byte{'1'}, 0o644)
	if err != nil {
		logger.Error("failed to send SIGKILL", zap.String("cgroup", cgroupPath), zap.Error(err))

		return err
	}

	for _, pidfd := range pidfds {
		_, _, errno := syscall.Syscall(unix.SYS_PROCESS_MRELEASE, uintptr(pidfd), uintptr(0), uintptr(0))
		if errno != 0 && errno != syscall.ESRCH {
			// FIXME: tolerate some errors esp given that some processes might have been freed already.
			logger.Error("failed to call mrelease", zap.Int("errno", int(errno)))

			continue
		}
	}

	return nil
}
