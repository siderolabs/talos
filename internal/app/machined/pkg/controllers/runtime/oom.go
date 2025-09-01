// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
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
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
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
		constants.DefaultOOMTriggerExpression,
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
		constants.DefaultOOMCgroupRankingExpression,
		celenv.OOMCgroupScoring(),
	))
})

const defaultSampleInterval = 500 * time.Millisecond

// Run implements controller.Controller interface.
//
//nolint:gocyclo
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

		evalContext := map[string]any{
			"time_since_trigger": time.Since(ctrl.ActionTriggered),
		}

		trigger, err := oom.EvaluateTrigger(triggerExpr, evalContext, ctrl.CgroupRoot)
		if err != nil {
			logger.Error("cannot evaluate OOM trigger expression", zap.Error(err))

			continue
		}

		// TODO: evaluate on different cgroups, not only root. E.g. action when podruntime experiences high PSI.
		if trigger {
			ctrl.ActionTriggered = time.Now()
			ctrl.OomAction(logger, ctrl.CgroupRoot, scoringExpr)
		}
	}
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
func (ctrl *OOMController) OomAction(logger *zap.Logger, root string, scoringExpr cel.Expression) {
	logger.Info("OOM controller triggered")

	ranking := oom.RankCgroups(logger, root, scoringExpr)

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

	err := reapCg(logger, cgroupToKill.Path)
	if err != nil {
		logger.Error("cannot reap cgroup", zap.String("cgroup", cgroupToKill.Path), zap.Error(err))
	}
}

func reapCg(logger *zap.Logger, cgroupPath string) error {
	logger.Warn("Sending SIGKILL to cgroup", zap.String("cgroup", cgroupPath))

	processes := oom.ListCgroupProcs(cgroupPath)
	logger.Info("victim processes:", zap.Any("processes", processes))

	// Open pidfd's of all the processes in cgroup to accelerate kernel
	// garbage-collecting those processes via mrelease.
	pidfds := []int{}

	for _, pid := range processes {
		// pidfd is always opened with CLOEXEC:
		// https://github.com/torvalds/linux/blob/bf40f4b87761e2ec16efc8e49b9ca0d81f4115d8/kernel/pid.c#L637
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
