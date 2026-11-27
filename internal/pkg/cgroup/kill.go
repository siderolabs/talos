// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Process is a process found in a cgroup.
type Process struct {
	// Cgroup is the cgroup directory the process is a member of.
	Cgroup  string
	Comm    string
	Cmdline string
	PID     int
}

// String implements fmt.Stringer.
func (p Process) String() string {
	return fmt.Sprintf("pid %d (%s) in %s cmdline=[%s]", p.PID, p.Comm, p.Cgroup, p.Cmdline)
}

// killPollInterval is how often KillProcesses checks whether the killed processes are gone.
const killPollInterval = 50 * time.Millisecond

// KillProcesses kills all processes in the cgroup v2 directory dir and its descendants, and waits
// for them to exit.
//
// The processes are killed with cgroup.kill, which also catches processes forked while the kill is
// in progress. It returns the processes found in the cgroup just before the kill, so that the caller
// can report what was left running.
//
// A missing cgroup has no processes. If the processes don't exit before ctx is done, an error is
// returned along with the processes found.
func KillProcesses(ctx context.Context, dir string) ([]Process, error) {
	processes, err := ListProcesses(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	if len(processes) == 0 {
		return nil, nil
	}

	if err = os.WriteFile(filepath.Join(dir, "cgroup.kill"), []byte("1"), 0o644); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// the cgroup was removed after the processes were listed, so they are gone
			return processes, nil
		}

		return processes, fmt.Errorf("error killing processes in cgroup %s: %w", dir, err)
	}

	return processes, waitUnpopulated(ctx, dir)
}

// waitUnpopulated waits for the cgroup and all of its descendants to have no live processes.
func waitUnpopulated(ctx context.Context, dir string) error {
	ticker := time.NewTicker(killPollInterval)
	defer ticker.Stop()

	for {
		populated, err := isPopulated(dir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}

			return err
		}

		if !populated {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("processes in cgroup %s are still running after the kill: %w", dir, ctx.Err())
		case <-ticker.C:
		}
	}
}

// ListProcesses lists the processes in the cgroup v2 directory dir and its descendants.
//
// Processes which exit while they are being listed are skipped.
func ListProcesses(dir string) ([]Process, error) {
	var processes []Process

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// a child cgroup removed while walking the tree is not an error, but the root cgroup must exist
			if path != dir && errors.Is(err, fs.ErrNotExist) {
				return nil
			}

			return err
		}

		if !d.IsDir() {
			return nil
		}

		contents, err := os.ReadFile(filepath.Join(path, "cgroup.procs"))
		if err != nil {
			if path != dir && errors.Is(err, fs.ErrNotExist) {
				return nil
			}

			return err
		}

		for field := range strings.FieldsSeq(string(contents)) {
			pid, err := strconv.Atoi(field)
			if err != nil {
				return fmt.Errorf("error parsing %s/cgroup.procs: %w", path, err)
			}

			processes = append(processes, describeProcess(path, pid))
		}

		return nil
	})

	return processes, err
}

func describeProcess(cgroupDir string, pid int) Process {
	process := Process{
		Cgroup: cgroupDir,
		PID:    pid,
		Comm:   "<unknown>",
	}

	procDir := filepath.Join("/proc", strconv.Itoa(pid))

	if comm, err := os.ReadFile(filepath.Join(procDir, "comm")); err == nil {
		process.Comm = strings.TrimSpace(string(comm))
	}

	if cmdline, err := os.ReadFile(filepath.Join(procDir, "cmdline")); err == nil {
		process.Cmdline = string(bytes.ReplaceAll(bytes.TrimRight(cmdline, "\x00"), []byte{0}, []byte{' '}))
	}

	return process
}

// isPopulated reports whether the cgroup or any of its descendants has live processes.
func isPopulated(dir string) (bool, error) {
	f, err := os.Open(filepath.Join(dir, "cgroup.events"))
	if err != nil {
		return false, err
	}

	defer f.Close() //nolint:errcheck

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), " ")
		if ok && key == "populated" {
			return value != "0", nil
		}
	}

	if err = scanner.Err(); err != nil {
		return false, err
	}

	return false, fmt.Errorf("no populated key in %s/cgroup.events", dir)
}
