// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysfs

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/siderolabs/gen/panicsafe"
	"golang.org/x/sync/errgroup"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// CPUFreqStats returns the current, minimum and maximum frequency (in kHz) and the governor of
// each online CPU, in CPU number order, read from sysfs mounted at root.
//
// A CPU which has no cpufreq policy, or whose frequency cannot be read, gets empty stats.
func CPUFreqStats(root string) ([]*machine.CPUFreqStats, error) {
	cpuDir := filepath.Join(root, "devices/system/cpu")

	online, err := ReadFile(filepath.Join(cpuDir, "online"))
	if err != nil {
		return nil, err
	}

	set, err := cpuset.Parse(strings.TrimSpace(string(online)))
	if err != nil {
		return nil, fmt.Errorf("error parsing online CPUs: %w", err)
	}

	cpus := set.List()
	if len(cpus) == 0 {
		return nil, errors.New("no online CPUs found")
	}

	stats := make([]*machine.CPUFreqStats, len(cpus))

	// Reading a CPU's current frequency might need an IPI to it, so read the CPUs in parallel.
	var eg errgroup.Group

	eg.SetLimit(runtime.GOMAXPROCS(0))

	for i, cpu := range cpus {
		// this runs in machined (PID 1), so a panic here would bring down the whole node
		eg.Go(panicsafe.RunErrF(func() error {
			var err error

			stats[i], err = readCPUFreqStats(filepath.Join(cpuDir, "cpu"+strconv.Itoa(cpu), "cpufreq"))

			return err
		}))
	}

	if err = eg.Wait(); err != nil {
		return nil, err
	}

	return stats, nil
}

func readCPUFreqStats(dir string) (*machine.CPUFreqStats, error) {
	var stats machine.CPUFreqStats

	for _, attr := range []struct {
		name string
		dest *uint64
	}{
		{"cpuinfo_cur_freq", &stats.CurrentFrequency},
		{"cpuinfo_min_freq", &stats.MinimumFrequency},
		{"cpuinfo_max_freq", &stats.MaximumFrequency},
	} {
		value, err := ReadFile(filepath.Join(dir, attr.name))
		if err != nil {
			if isUnavailable(err) {
				return &machine.CPUFreqStats{}, nil
			}

			return nil, err
		}

		// the kernel prints "<unknown>" for cpuinfo_cur_freq when the driver can't read it
		*attr.dest, err = strconv.ParseUint(strings.TrimSpace(string(value)), 10, 64)
		if err != nil {
			return &machine.CPUFreqStats{}, nil //nolint:nilerr
		}
	}

	governor, err := ReadFile(filepath.Join(dir, "scaling_governor"))
	if err != nil && !isUnavailable(err) {
		return nil, err
	}

	stats.Governor = strings.TrimSpace(string(governor))

	return &stats, nil
}

// isUnavailable reports whether err means the attribute has no value to report: the CPU has no
// cpufreq policy or the driver doesn't expose the attribute (ENOENT), the attribute is not
// readable (EACCES), the driver can't read the value right now (EAGAIN), or the policy is
// inactive (EBUSY).
func isUnavailable(err error) bool {
	return errors.Is(err, fs.ErrNotExist) ||
		errors.Is(err, fs.ErrPermission) ||
		errors.Is(err, syscall.EAGAIN) ||
		errors.Is(err, syscall.EBUSY)
}
