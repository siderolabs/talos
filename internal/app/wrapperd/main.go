// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package wrapperd provides a wrapper for running services.
package wrapperd

import (
	"errors"
	"flag"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/containerd/cgroups"
	cgroupsv2 "github.com/containerd/cgroups/v2"
	"github.com/containerd/containerd/sys"
	"github.com/siderolabs/gen/slices"
	"golang.org/x/sys/unix"
	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var (
	name        string
	droppedCaps string
	cgroupPath  string
	oomScore    int
)

// Main is the entrypoint into /sbin/wrapperd.
// nolint: gocyclo
func Main() {
	flag.StringVar(&name, "name", "", "process name")
	flag.StringVar(&droppedCaps, "dropped-caps", "", "comma-separated list of capabilities to drop")
	flag.StringVar(&cgroupPath, "cgroup-path", "", "cgroup path to use")
	flag.IntVar(&oomScore, "oom-score", 0, "oom score to set")
	flag.Parse()

	if droppedCaps != "" {
		caps := strings.Split(droppedCaps, ",")
		dropCaps := slices.Map(caps, func(c string) cap.Value {
			capability, err := cap.FromName(c)
			if err != nil {
				log.Fatalf("failed to parse capability: %v", err)
			}

			return capability
		})

		// drop capabilities
		iab := cap.IABGetProc()
		if err := iab.SetVector(cap.Bound, true, dropCaps...); err != nil {
			log.Fatalf("failed to set capabilities: %v", err)
		}

		if err := iab.SetProc(); err != nil {
			log.Fatalf("failed to apply capabilities: %v", err)
		}
	}

	var (
		cgv1 cgroups.Cgroup
		cgv2 *cgroupsv2.Manager
		err  error
	)

	currentPid := os.Getpid()

	// load the cgroup
	if cgroupPath != "" {
		if cgroups.Mode() == cgroups.Unified {
			cgv2, err = cgroupsv2.LoadManager(constants.CgroupMountPath, cgroupPath)
			if err != nil {
				log.Fatalf("failed to load cgroup %s: %v", cgroupPath, err)
			}
		} else {
			cgv1, err = cgroups.Load(cgroups.V1, cgroups.StaticPath(cgroupPath))
			if err != nil {
				log.Fatalf("failed to load cgroup %s: %v", cgroupPath, err)
			}
		}
	}

	if oomScore != 0 {
		if err := sys.AdjustOOMScore(currentPid, oomScore); err != nil {
			log.Fatalf("Failed to change OOMScoreAdj of process %s to %d", name, oomScore)
		}
	}

	if cgroupPath != "" {
		// put the process into the cgroup and record failure (if any)
		if cgroups.Mode() == cgroups.Unified {
			if err := cgv2.AddProc(uint64(currentPid)); err != nil && !errors.Is(err, syscall.ESRCH) { // ignore "no such process" error
				log.Fatalf("Failed to move process %s to cgroup: %s", name, err)
			}
		} else {
			if err := cgv1.Add(cgroups.Process{
				Pid: currentPid,
			}); err != nil && !errors.Is(err, syscall.ESRCH) { // ignore "no such process" error
				log.Fatalf("Failed to move process %s to cgroup: %s", name, err)
			}
		}
	}

	if err := unix.Exec(flag.Args()[0], flag.Args()[0:], os.Environ()); err != nil {
		log.Fatalf("failed to exec: %v", err)
	}
}
