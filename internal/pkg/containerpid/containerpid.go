// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package containerpid translates container PIDs reported by containerd into the PID
// namespace of the calling process.
package containerpid

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// ErrGone is returned by [Resolver.Resolve] when no process in the container cgroup (or any cgroup
// below it) matches the PID containerd reported: the container init exited between the task start
// and the lookup, and only its children (if any) are left behind.
var ErrGone = errors.New("container process is gone")

// runtimePIDNamespaceTimeout bounds the query for the PID namespace of the runtime: it is made on
// task start, on contexts which are otherwise not bounded at all.
const runtimePIDNamespaceTimeout = 10 * time.Second

// maxReportedSkips bounds how many per-process failures end up in a [Resolver.Resolve] error.
const maxReportedSkips = 3

// PIDNamespacesFunc returns the inode numbers of the PID namespaces of a process, innermost
// first, up to and including the PID namespace of the caller.
//
// pid is either a numeric PID or "self". If the process is gone, the error wraps
// [io/fs.ErrNotExist].
type PIDNamespacesFunc func(pid string) ([]uint64, error)

// Resolver resolves container init PIDs by reading cgroupfs and procfs.
type Resolver struct {
	cgroupMountPath string
	procPath        string
	pidNamespaces   PIDNamespacesFunc
	readOwnPIDNS    func() (uint64, error)

	// The PID namespace of the caller never changes, so it is read once; zero is "not read yet".
	ownPIDNS atomic.Uint64
}

var defaultResolver = sync.OnceValue(NewResolver)

// DefaultResolver returns the shared resolver reading the default /sys/fs/cgroup and /proc mount
// points.
func DefaultResolver() *Resolver {
	return defaultResolver()
}

// NewResolver initializes a resolver reading the default /sys/fs/cgroup and /proc mount points.
func NewResolver() *Resolver {
	return NewResolverWithPaths(constants.CgroupMountPath, "/proc", nil)
}

// NewResolverWithPaths initializes a resolver reading non-default mount points.
//
// If pidNamespaces is nil, PID namespaces are read from procPath with NS_GET_PARENT.
func NewResolverWithPaths(cgroupMountPath, procPath string, pidNamespaces PIDNamespacesFunc) *Resolver {
	r := &Resolver{
		cgroupMountPath: cgroupMountPath,
		procPath:        procPath,
		pidNamespaces:   pidNamespaces,
	}

	if pidNamespaces != nil {
		r.readOwnPIDNS = func() (uint64, error) {
			namespaces, err := pidNamespaces("self")
			if err != nil {
				return 0, err
			}

			return namespaces[0], nil
		}

		return r
	}

	r.pidNamespaces = func(pid string) ([]uint64, error) {
		return ReadPIDNamespaces(filepath.Join(procPath, pid, "ns", "pid"))
	}

	// A plain stat: the namespace of the caller is the only one needed here, and every containerd
	// task start goes through this, so it should not depend on anything more than procfs.
	r.readOwnPIDNS = func() (uint64, error) {
		var st unix.Stat_t

		nsPath := filepath.Join(procPath, "self", "ns", "pid")

		if err := unix.Stat(nsPath, &st); err != nil {
			return 0, &fs.PathError{Op: "stat", Path: nsPath, Err: err}
		}

		return st.Ino, nil
	}

	return r
}

// RuntimePIDNamespace returns the inode number of the PID namespace the containerd instance
// behind client runs in, as needed by [Resolver.Resolve].
func RuntimePIDNamespace(ctx context.Context, client *containerd.Client) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, runtimePIDNamespaceTimeout)
	defer cancel()

	resp, err := client.IntrospectionService().Server(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to query containerd server info: %w", err)
	}

	if resp.GetPidns() == 0 {
		return 0, errors.New("containerd did not report its PID namespace")
	}

	return resp.GetPidns(), nil
}

// ResolveTask translates the PID of a started task of the containerd instance behind client into
// the PID namespace this process runs in, see [Resolver.Resolve].
//
// The runtime is asked for its PID namespace on every call rather than once: a containerd
// restarted in a recreated sandbox comes back in a different one.
//
// A failure means the PID is not known, not that the task is broken: whether the task is worth
// keeping without a PID is up to the caller.
func ResolveTask(ctx context.Context, client *containerd.Client, cgroupPath string, task containerd.Task) (int32, error) {
	runtimePIDNS, err := RuntimePIDNamespace(ctx, client)
	if err != nil {
		return 0, err
	}

	return DefaultResolver().Resolve(cgroupPath, runtimePIDNS, task.Pid())
}

// Resolve translates the container init PID reported by containerd into the PID namespace
// this process runs in.
//
// containerd reports task PIDs in its own PID namespace (runtimePIDNS, as returned by
// [RuntimePIDNamespace]). When the CRI containerd runs under sandboxd, that is the sandbox PID
// namespace, not the one of machined, and a PID namespace can only be entered downwards, so the
// translation has to happen on this side. If the runtime shares the PID namespace of the caller,
// runtimePID is already the right one and is returned as is.
//
// Otherwise the container init is looked up among the processes of its cgroup (cgroupPath,
// relative to the cgroupfs mount point) and of the cgroups below it, as an init is free to move
// itself into a child cgroup: cgroupfs renders PIDs in the PID namespace of the process reading
// it, so these PIDs are directly usable by the caller. The sandbox namespace is created with
// CLONE_NEWPID|CLONE_NEWNS only (see sandboxd's beforeSandboxdExec), so no cgroup namespace is
// involved and the cgroup path means the same thing on both sides.
//
// For each process, the chain of PID namespaces it is nested in tells which NSpid entry is the
// PID as seen by the runtime: that does not depend on whether the container got a PID namespace
// of its own or shares the one of the runtime, and processes outside the runtime's namespace are
// never matched by accident.
//
// A process which cannot be inspected is skipped, so that it does not hide the init next to it.
// If the init is not found, the error is [ErrGone] when every process was inspected (the init is
// not, or no longer, there), and the reasons the others were skipped for otherwise.
func (r *Resolver) Resolve(cgroupPath string, runtimePIDNS uint64, runtimePID uint32) (int32, error) {
	own, err := r.ownPIDNamespace()
	if err != nil {
		return 0, fmt.Errorf("failed to read own PID namespace: %w", err)
	}

	if own == runtimePIDNS {
		return int32(runtimePID), nil
	}

	if cgroupPath == "" {
		return 0, errors.New("container runs in another PID namespace, but its cgroup is not known")
	}

	cgroupDir, err := r.cgroupDir(cgroupPath)
	if err != nil {
		return 0, err
	}

	scan := treeScan{
		resolver:     r,
		root:         cgroupDir,
		runtimePIDNS: runtimePIDNS,
		runtimePID:   runtimePID,
	}

	if err = filepath.WalkDir(cgroupDir, scan.visit); err != nil {
		return 0, err
	}

	switch {
	case scan.found != 0:
		return scan.found, nil
	case len(scan.skipped) == 0:
		return 0, fmt.Errorf("%w: no process with PID %d (as seen by containerd) found under %s", ErrGone, runtimePID, cgroupDir)
	default:
		return 0, fmt.Errorf(
			"no process with PID %d (as seen by containerd) found under %s, and %d could not be inspected: %w",
			runtimePID, cgroupDir, len(scan.skipped), errors.Join(scan.skipped[:min(len(scan.skipped), maxReportedSkips)]...),
		)
	}
}

// ownPIDNamespace returns the inode number of the PID namespace of the caller.
func (r *Resolver) ownPIDNamespace() (uint64, error) {
	if own := r.ownPIDNS.Load(); own != 0 {
		return own, nil
	}

	own, err := r.readOwnPIDNS()
	if err != nil {
		return 0, err
	}

	r.ownPIDNS.Store(own)

	return own, nil
}

// cgroupDir locates the directory of the cgroup.
//
// On the unified hierarchy that is the path under the mount point. With cgroup v1 there is one
// such directory per controller hierarchy, all of them listing the same processes, and any will do.
func (r *Resolver) cgroupDir(cgroupPath string) (string, error) {
	const procs = "cgroup.procs"

	dir := filepath.Join(r.cgroupMountPath, cgroupPath)

	_, err := os.Stat(filepath.Join(dir, procs))
	if err == nil {
		return dir, nil
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	// cgroup.controllers only exists at the root of the unified hierarchy
	if _, statErr := os.Stat(filepath.Join(r.cgroupMountPath, "cgroup.controllers")); statErr == nil {
		return "", err
	}

	hierarchies, readErr := os.ReadDir(r.cgroupMountPath)
	if readErr != nil {
		return "", err
	}

	for _, hierarchy := range hierarchies {
		v1Dir := filepath.Join(r.cgroupMountPath, hierarchy.Name(), cgroupPath)

		if _, statErr := os.Stat(filepath.Join(v1Dir, procs)); statErr == nil {
			return v1Dir, nil
		}
	}

	return "", err
}

// treeScan looks for the process with runtimePID in the PID namespace runtimePIDNS in the cgroup
// root and the cgroups below it.
type treeScan struct {
	resolver     *Resolver
	root         string
	runtimePIDNS uint64
	runtimePID   uint32

	// found is the PID of the process, or zero if it is not there.
	found int32
	// skipped is why some processes (or child cgroups) could not be inspected.
	skipped []error
}

// visit implements [io/fs.WalkDirFunc].
//
// Only a failure to read the root cgroup stops the scan: child cgroups come and go.
func (s *treeScan) visit(path string, d fs.DirEntry, err error) error {
	if err == nil {
		if !d.IsDir() {
			return nil
		}

		err = s.scanProcs(filepath.Join(path, "cgroup.procs"))
	}

	switch {
	case err == nil:
		if s.found != 0 {
			return fs.SkipAll
		}
	case path == s.root:
		return err
	case !errors.Is(err, fs.ErrNotExist):
		s.skipped = append(s.skipped, err)
	}

	return nil
}

// scanProcs looks for the process among the ones listed in a cgroup.procs file.
func (s *treeScan) scanProcs(procsPath string) error {
	contents, err := os.ReadFile(procsPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", procsPath, err)
	}

	for field := range strings.FieldsSeq(string(contents)) {
		candidate, err := strconv.ParseInt(field, 10, 32)
		if err != nil {
			return fmt.Errorf("failed to parse PID %q from %s: %w", field, procsPath, err)
		}

		matched, err := s.resolver.match(int32(candidate), s.runtimePIDNS, s.runtimePID)

		switch {
		case errors.Is(err, fs.ErrNotExist):
			// the process exited between reading the cgroup and inspecting it
		case err != nil:
			s.skipped = append(s.skipped, err)
		case matched:
			s.found = int32(candidate)

			return nil
		}
	}

	return nil
}

// match reports whether the process candidate has runtimePID in the PID namespace runtimePIDNS.
func (r *Resolver) match(candidate int32, runtimePIDNS uint64, runtimePID uint32) (bool, error) {
	nsPIDs, err := r.readNSPIDs(candidate)
	if err != nil {
		return false, err
	}

	// The runtime's namespace is below the caller's one, which the first entry is for. Almost every
	// process is ruled out right here, without walking its namespaces.
	if !slices.Contains(nsPIDs[1:], int32(runtimePID)) {
		return false, nil
	}

	namespaces, err := r.pidNamespaces(strconv.Itoa(int(candidate)))
	if err != nil {
		return false, fmt.Errorf("failed to read PID namespaces of %d: %w", candidate, err)
	}

	level := slices.Index(namespaces, runtimePIDNS)
	if level < 0 {
		// not visible from the runtime's PID namespace at all
		return false, nil
	}

	// NSpid lists the PID in each namespace from the caller's one down, the namespace chain goes the
	// other way round; both cover the same namespaces, from the process up to the caller.
	if len(nsPIDs) != len(namespaces) {
		return false, fmt.Errorf("process %d is nested in %d PID namespaces, but NSpid has %d entries", candidate, len(namespaces), len(nsPIDs))
	}

	return nsPIDs[len(nsPIDs)-1-level] == int32(runtimePID), nil
}

// readNSPIDs returns the NSpid entries of the process.
//
// If the process is gone, the returned error wraps [io/fs.ErrNotExist].
func (r *Resolver) readNSPIDs(pid int32) ([]int32, error) {
	statusPath := filepath.Join(r.procPath, strconv.Itoa(int(pid)), "status")

	// /proc/<pid>/status is small enough to read in one go.
	contents, err := os.ReadFile(statusPath)
	if err != nil {
		return nil, normalizeGone(err)
	}

	nsPIDs, err := parseNSPIDs(contents)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", statusPath, err)
	}

	return nsPIDs, nil
}

// parseNSPIDs extracts the NSpid entries.
func parseNSPIDs(contents []byte) ([]int32, error) {
	for line := range strings.Lines(string(contents)) {
		rest, ok := strings.CutPrefix(line, "NSpid:")
		if !ok {
			continue
		}

		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return nil, errors.New("NSpid field is empty")
		}

		nsPIDs := make([]int32, 0, len(fields))

		for _, field := range fields {
			nsPID, err := strconv.ParseInt(field, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("failed to parse NSpid entry %q: %w", field, err)
			}

			nsPIDs = append(nsPIDs, int32(nsPID))
		}

		return nsPIDs, nil
	}

	return nil, errors.New("no NSpid field found")
}

// ReadPIDNamespaces implements [PIDNamespacesFunc] for a /proc/<pid>/ns/pid path.
//
// The chain is walked with NS_GET_PARENT, which stops (with EPERM) at the PID namespace of the
// caller: the namespaces above it are not visible.
func ReadPIDNamespaces(nsPath string) ([]uint64, error) {
	fd, err := unix.Open(nsPath, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, normalizeGone(&fs.PathError{Op: "open", Path: nsPath, Err: err})
	}

	var namespaces []uint64

	for {
		var st unix.Stat_t

		if err = unix.Fstat(fd, &st); err != nil {
			unix.Close(fd) //nolint:errcheck

			return nil, fmt.Errorf("failed to stat PID namespace of %s: %w", nsPath, err)
		}

		namespaces = append(namespaces, st.Ino)

		parent, err := unix.IoctlRetInt(fd, unix.NS_GET_PARENT)

		unix.Close(fd) //nolint:errcheck

		if errors.Is(err, unix.EPERM) {
			return namespaces, nil
		}

		if err != nil {
			return nil, fmt.Errorf("failed to get parent PID namespace of %s: %w", nsPath, err)
		}

		fd = parent
	}
}

// normalizeGone makes a process which goes away mid-read report [io/fs.ErrNotExist]: procfs
// reports ESRCH instead of ENOENT in that case, and callers need a single signal for it.
func normalizeGone(err error) error {
	if errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("%w: %w", fs.ErrNotExist, err)
	}

	return err
}
