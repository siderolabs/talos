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
	"syscall"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/oom"
	"github.com/siderolabs/talos/internal/pkg/cgroups"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const (
	sampleInterval  = 5000 * time.Millisecond
	mempressureProp = "memory.pressure"
	pressureType    = "full"
	pressureSpan    = "avg10"
	psiThresh       = 12
	cooldownTimeout = 500 * time.Millisecond
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

func (ctrl *OOMController) defaultScoringExpr() cel.Expression {
	return cel.MustExpression(cel.ParseDoubleExpression(
		`memory_max.hasValue() ? 0.0 : ({Besteffort: 1.0, Guaranteed: 0.0, Burstable: 0.5}[class] * double(memory_current.orValue(0u)) / double(memory_peak.orValue(0u) - memory_current.orValue(0u)))`,
		celenv.OOMCgroupScoring(),
	))
}

// Run implements controller.Controller interface.
func (ctrl *OOMController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	scoringExpr := ctrl.defaultScoringExpr()

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

			scoringExpr = ctrl.defaultScoringExpr()

			if cfg != nil {
				if oomCfg := cfg.Config().OOMConfig(); oomCfg != nil {
					if expr, ok := oomCfg.CgroupRankingExpression().Get(); ok {
						scoringExpr = expr
					}
				}
			}
		case <-tickerC:
		}

		node, err := cgroups.GetCgroupProperty(constants.CgroupMountPath, mempressureProp)
		if err != nil {
			fmt.Println("cannot read memory pressure", err)
			continue
		}

		fmt.Println(node.MemoryPressure)

		spans, ok := node.MemoryPressure[pressureType]
		if !ok {
			fmt.Println("cannot find memory pressure type:", pressureType)
			continue
		}

		value, ok := spans[pressureSpan]
		if !ok {
			fmt.Println("cannot find memory pressure span:", pressureSpan)
			continue
		}

		if !value.IsSet || value.IsMax {
			continue
		}

		val, err := strconv.ParseFloat(value.String(), 32)
		if err != nil {
			fmt.Println("cannot parse memory pressure:", pressureSpan, err)
			continue
		}
		fmt.Println("monitoring", value.String(), val, err)

		if val > psiThresh && time.Since(ctrl.ActionTriggered) > cooldownTimeout {
			ctrl.ActionTriggered = time.Now()
			ctrl.OomAction(logger, scoringExpr)
		}
	}
}

// OomAction
func (ctrl *OOMController) OomAction(logger *zap.Logger, scoringExpr cel.Expression) {
	fmt.Println("OOM action!")

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
			fmt.Println("cannot list cgroup members", cg.dir, err)
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

			score, err := cgroup.CalculateScore(&scoringExpr)
			if err != nil {
				logger.Error("cannot calculate score for cgroup",
					zap.String("dir", leafDir), zap.Error(err),
				)

				continue
			}

			ranking[cgroup] = score
		}
	}

	if len(ranking) == 0 {
		return
	}

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

	var (
		maxScore     float64 = math.Inf(-1)
		cgroupToKill oom.RankedCgroup
	)

	for cgroup, score := range ranking {
		if score > maxScore {
			maxScore = score
			cgroupToKill = cgroup
		}
	}

	fmt.Println("SENDING SIGKILL TO CGROUP", filepath.Join(cgroupToKill.Path, "cgroup.kill"))

	err := ctrl.reapCg(cgroupToKill.Path)
	if err != nil {
		fmt.Println("cannot reap cgroup", cgroupToKill.Path, err)
	}
}

func (ctrl *OOMController) reapCg(cgroupPath string) error {
	processes := []int{}
	// Ignore errors, find as many processes as possible
	filepath.WalkDir(cgroupPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		node, err := cgroups.GetCgroupProperty(path, "cgroup.procs")
		if err != nil {
			return nil
		}

		fmt.Println("visiting:", path)
		for _, p := range node.CgroupProcs {
			processes = append(processes, int(p.Val))
		}

		return nil
	})
	fmt.Println("victim processes:", processes)

	pidfds := []int{}
	for _, pid := range processes {
		pidfd, err := unix.PidfdOpen(pid, 0)
		if err != nil {
			fmt.Println("failed to open pidfd", pid, err)
			continue
		}
		defer unix.Close(pidfd)
		pidfds = append(pidfds, pidfd)
	}

	os.WriteFile(filepath.Join(cgroupPath, "cgroup.kill"), []byte{'1'}, 0o644)

	for _, pidfd := range pidfds {
		_, _, errno := syscall.Syscall(unix.SYS_PROCESS_MRELEASE, uintptr(pidfd), uintptr(0), uintptr(0))
		if errno != 0 && errno != syscall.ESRCH {
			fmt.Println("failed to call mrelease", errno)
			continue
		}
		fmt.Println("mreleased")
	}

	return nil
}
