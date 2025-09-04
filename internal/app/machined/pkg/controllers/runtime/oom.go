// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/cgroups"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	sampleInterval  = 5000 * time.Millisecond
	mempressureProp = "memory.pressure"
	pressureType    = "full"
	pressureSpan    = "avg10"
	psiThresh       = 12
	cooldownTimeout = 500 * time.Millisecond
)

// Higher value corresponds to a more important cgroup
const (
	OomCgroupClassBesteffort = iota
	OomCgroupClassBurstable
	OomCgroupClassGuaranteed
	OomCgroupClassPodruntime
	OomCgroupClassSystem
)

type oomRankedCgroup struct {
	Class         int
	Path          string
	MemoryCurrent int64
	MemoryPeak    int64
	MemoryMax     int64
}

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
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *OOMController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *OOMController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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
			ctrl.OomAction(logger)
		}
	}
}

// OomAction
func (ctrl *OOMController) OomAction(logger *zap.Logger) {
	fmt.Println("OOM action!")

	ranking := []oomRankedCgroup{}

	for _, cg := range []struct {
		dir   string
		class int
	}{
		{"kubepods/besteffort", OomCgroupClassBesteffort},
		{"kubepods/burstable", OomCgroupClassBurstable},
		{"kubepods/guaranteed", OomCgroupClassGuaranteed},
		{"podruntime", OomCgroupClassPodruntime},
		{"system", OomCgroupClassSystem},
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
					fmt.Println("cannot read property for cgroup", leafDir, prop, err)
					continue
				}
			}

			ranking = append(ranking, oomRankedCgroup{
				Path:          leafDir,
				Class:         cg.class,
				MemoryCurrent: node.MemoryCurrent.Val,
				MemoryPeak:    node.MemoryPeak.Val,
				MemoryMax:     node.MemoryMax.Val,
			})
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
	//
	// TODO: implement CEL or other configurable ranking method
	sort.Slice(ranking, func(i int, j int) bool {
		a, b := ranking[i], ranking[j]
		if a.Class == b.Class {
			return a.MemoryCurrent > b.MemoryCurrent
		}

		return a.Class < b.Class
	})

	fmt.Println(ranking)
	fmt.Println("SENDING SIGKILL TO CGROUP", filepath.Join(ranking[0].Path, "cgroup.kill"))

	err := ctrl.reapCg(ranking[0].Path)
	if err != nil {
		fmt.Println("cannot reap cgroup", ranking[0].Path, err)
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
