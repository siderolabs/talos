// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package wrapperd provides a wrapper for running services.
package wrapperd

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/containerd/containerd/v2/pkg/sys"
	"github.com/siderolabs/gen/xslices"
	"golang.org/x/sys/unix"
	"kernel.org/pub/linux/libs/security/libcap/cap"

	krnl "github.com/siderolabs/talos/pkg/kernel"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
)

var (
	name        string
	droppedCaps string
	cgroupPath  string
	oomScore    int
	uid         int
)

// Main is the entrypoint into /sbin/wrapperd.
//
//nolint:gocyclo
func Main() {
	flag.StringVar(&name, "name", "", "process name")
	flag.StringVar(&droppedCaps, "dropped-caps", "", "comma-separated list of capabilities to drop")
	flag.StringVar(&cgroupPath, "cgroup-path", "", "cgroup path to use")
	flag.IntVar(&oomScore, "oom-score", 0, "oom score to set")
	flag.IntVar(&uid, "uid", 0, "uid to set for the process")
	flag.Parse()

	currentPid := os.Getpid()

	if oomScore != 0 {
		if err := sys.AdjustOOMScore(currentPid, oomScore); err != nil {
			log.Fatalf("Failed to change OOMScoreAdj of process %s to %d", name, oomScore)
		}
	}

	// load the cgroup and put the process into the cgroup
	if cgroupPath != "" {
		if cgroups.Mode() == cgroups.Unified {
			cgv2, err := cgroup2.Load(cgroupPath)
			if err != nil {
				log.Fatalf("failed to load cgroup %s: %v", cgroupPath, err)
			}

			if err := cgv2.AddProc(uint64(currentPid)); err != nil {
				log.Fatalf("Failed to move process %s to cgroup: %v", name, err)
			}
		} else {
			cgv1, err := cgroup1.Load(cgroup1.StaticPath(cgroupPath))
			if err != nil {
				log.Fatalf("failed to load cgroup %s: %v", cgroupPath, err)
			}

			if err := cgv1.Add(cgroup1.Process{
				Pid: currentPid,
			}); err != nil {
				log.Fatalf("Failed to move process %s to cgroup: %v", name, err)
			}
		}
	}

	prop, err := krnl.ReadParam(&kernel.Param{Key: "proc.sys.kernel.kexec_load_disabled"})
	if v := strings.TrimSpace(string(prop)); err == nil && v != "0" {
		log.Printf("kernel.kexec_load_disabled is %v, skipping dropping capabilities", v)
	} else if droppedCaps != "" {
		caps := strings.Split(droppedCaps, ",")
		dropCaps := xslices.Map(caps, func(c string) cap.Value {
			capability, capErr := cap.FromName(c)
			if capErr != nil {
				log.Fatalf("failed to parse capability: %v", capErr)
			}

			return capability
		})

		// drop capabilities
		iab := cap.IABGetProc()
		if err = iab.SetVector(cap.Bound, true, dropCaps...); err != nil {
			log.Fatalf("failed to set capabilities: %v", err)
		}

		if err = iab.SetProc(); err != nil {
			log.Fatalf("failed to apply capabilities: %v", err)
		}
	}

	if uid > 0 {
		err = unix.Setuid(uid)
		if err != nil {
			log.Fatalf("failed to setuid: %v", err)
		}
	}

	if err := unix.Exec(flag.Args()[0], flag.Args()[0:], os.Environ()); err != nil {
		log.Fatalf("failed to exec: %v", err)
	}
}
