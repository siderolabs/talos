// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package debug

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/errdefs"
	ocispec "github.com/opencontainers/image-spec/identity"
	"github.com/siderolabs/go-cmd/pkg/cmd/proc/reaper"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/internal/pkg/hostns"
	"github.com/siderolabs/talos/internal/pkg/lookpath"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// runHostNsContainer handles PROFILE_HOST_NS: it builds a debug root in the HOST mount
// namespace — host / as an overlay lower with a disk-backed upper, the image's /nix
// overlaid in, and the live host mounts bound in — then execs the requested command
// chrooted into that root. Because it runs in the host mount namespace (no fork), host
// binaries work at their native paths and tools that manage host mounts (zpool, mount)
// affect the host; the overlay's scratch mounts are removed on teardown.
func runHostNsContainer( //nolint:gocyclo
	ctx context.Context,
	detachedCtx context.Context,
	client *containerd.Client,
	img containerd.Image,
	spec *machine.DebugContainerRunRequestSpec,
	srv grpc.BidiStreamingServer[machine.DebugContainerRunRequest, machine.DebugContainerRunResponse],
	containerID string,
	cgroupPath string,
) error {
	// Derive a cancelable context. When streamHostNs returns — client disconnect,
	// transport error, or child exit — we cancel it so the control loop kills the
	// child process group, then wait for the launch goroutine before teardown runs.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 1. Writable snapshot of the image (Prepare, not View): the read-only /nix source.
	// The default in-memory ("inmem") debug containerd roots on tmpfs, so session writes
	// (including new /nix store paths) are captured in disk-backed overlay uppers under
	// varBase instead. Removed by the deferred Remove below.
	snapshotKey := containerID + "-hostns-rw"

	diffIDs, err := img.RootFS(ctx)
	if err != nil {
		return fmt.Errorf("host-ns: get image rootfs: %w", err)
	}

	chainID := ocispec.ChainID(diffIDs).String()

	snapshotMounts, err := client.SnapshotService("").Prepare(ctx, snapshotKey, chainID)
	if err != nil {
		return fmt.Errorf("host-ns: prepare writable snapshot: %w", err)
	}

	defer func() {
		if rmErr := client.SnapshotService("").Remove(detachedCtx, snapshotKey); rmErr != nil && !errdefs.IsNotFound(rmErr) {
			log.Printf("host-ns: failed to remove snapshot %s: %v", snapshotKey, rmErr)
		}
	}()

	// 2. Per-session work dir on the EPHEMERAL /var disk. It holds both the image/merged
	// mount points and the overlay upper/work layers — everything under one directory,
	// on disk (not /run, which is tmpfs/RAM; the overlay writes, including the nix store,
	// must be disk-backed).
	workDir := filepath.Join(constants.DebugHostNsWorkdirBase, containerID)
	if err = os.MkdirAll(workDir, 0o700); err != nil {
		return fmt.Errorf("host-ns: create work dir: %w", err)
	}

	defer os.RemoveAll(workDir) //nolint:errcheck

	// 3. Build the debug root in the host mount namespace; teardown removes only our
	// scratch mounts, leaving host mounts (and anything the session created) intact.
	merged, teardown, err := hostns.Setup(snapshotMounts, workDir, workDir)
	if err != nil {
		return fmt.Errorf("host-ns: setup root: %w", err)
	}

	defer teardown() //nolint:errcheck

	// 4. Seed /etc files the squashfs lower doesn't carry.
	seedEtcFiles(merged)

	// 5. gRPC I/O streams.
	grpcStreamer, stdinR, stdoutW := newGrpcStreamWriter(srv)

	// 6. Command + args: with explicit args, args[0] is the executable (resolved
	// against PATH in launchInHostNs); otherwise default to the Nix bash.
	const defaultShell = "/nix/var/nix/profiles/default/bin/bash"

	var (
		shell   string
		cmdArgs []string
	)

	if args := spec.GetArgs(); len(args) > 0 {
		shell = args[0]
		cmdArgs = args
	} else {
		shell = defaultShell
		cmdArgs = []string{defaultShell}
	}

	// 7. Env: Nix profile on PATH (per-user profile first so nix-env installs win),
	// plus caller-supplied overrides.
	env := []string{
		"PATH=/root/.nix-profile/bin:/nix/var/nix/profiles/default/bin:/nix/var/nix/profiles/default/sbin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"NIX_SSL_CERT_FILE=/nix/var/nix/profiles/default/etc/ssl/certs/ca-bundle.crt",
		"TERM=xterm-256color",
		"HOME=/root",
	}

	for k, v := range spec.GetEnv() {
		env = append(env, k+"="+v)
	}

	// 8. Per-session cgroup dir as an fd: SysProcAttr.CgroupFD places the child into
	// the cgroup atomically at fork, avoiding a post-fork cgroup.procs write race.
	cgroupFd, err := os.Open(filepath.Join("/sys/fs/cgroup", cgroupPath))
	if err != nil {
		return fmt.Errorf("host-ns: open cgroup dir %s: %w", cgroupPath, err)
	}

	// 9. Control channel: carries signals and pty-resize events from the gRPC recv
	// loop into the goroutine that owns the child. Buffered so recv never blocks.
	controlC := make(chan hostNsControl, 16)

	exitC := make(chan int, 1)
	launchDone := make(chan struct{})

	go func() {
		defer close(launchDone)

		code, launchErr := launchInHostNs(ctx, merged, shell, cmdArgs, env, stdinR, stdoutW, spec.GetTty(), cgroupFd, controlC)
		cgroupFd.Close() //nolint:errcheck

		if launchErr != nil {
			log.Printf("host-ns: launch error: %v", launchErr)

			// Surface the failure to the client: it flows through the stdout pipe →
			// sendLoop → talosctl, so the user sees the cause, not a bare exit code.
			fmt.Fprintf(stdoutW, "host-ns: launch failed: %v\r\n", launchErr) //nolint:errcheck
		}

		// Close the stdout pipe so the send loop drains and the streaming coordinator
		// receives EOF before we send the exit code.
		grpcStreamer.stdoutW.Close() //nolint:errcheck

		exitC <- code
	}()

	streamErr := grpcStreamer.streamHostNs(ctx, exitC, controlC)

	// Kill the child process group (no-op if already exited), then wait for the launch
	// goroutine before the deferred teardown/cleanup runs.
	cancel()
	<-launchDone

	return streamErr
}

// seedEtcFiles writes into the overlay's /etc the files the raw squashfs lower does
// not carry: the host's live resolv.conf (otherwise DNS is broken), and a nix.conf so
// the package manager works without the nixbld build-users group or a sandbox.
func seedEtcFiles(merged string) {
	if hostResolv, readErr := os.ReadFile("/etc/resolv.conf"); readErr == nil && len(hostResolv) > 0 {
		resolvDst := filepath.Join(merged, "etc", "resolv.conf")

		if mkErr := os.MkdirAll(filepath.Dir(resolvDst), 0o755); mkErr == nil {
			os.WriteFile(resolvDst, hostResolv, 0o644) //nolint:errcheck
		}
	}

	nixConfDst := filepath.Join(merged, "etc", "nix", "nix.conf")
	if mkErr := os.MkdirAll(filepath.Dir(nixConfDst), 0o755); mkErr == nil {
		os.WriteFile(nixConfDst, []byte( //nolint:errcheck
			"build-users-group =\n"+
				"sandbox = false\n"+
				"experimental-features = nix-command flakes\n",
		), 0o644)
	}
}

// launchInHostNs execs the requested command chrooted into the prepared root (merged),
// wiring stdio (or a pty) and placing the child in the debug cgroup. Mount setup lives
// in package hostns; this only starts and supervises the child.
func launchInHostNs(
	ctx context.Context,
	merged string,
	shell string,
	args []string,
	env []string,
	stdin io.Reader,
	stdout io.Writer,
	tty bool,
	cgroupFd *os.File,
	controlC <-chan hostNsControl,
) (exitCode int, err error) {
	// Resolve the executable against PATH within the root: execve does not search PATH
	// for a bare command name (e.g. "zpool"), and we cannot LookPath in machined's
	// namespace (no /nix there). lookpath.InRoot resolves inside merged (RESOLVE_IN_ROOT),
	// handling host binaries and Nix profile symlinks alike.
	execPath, err := lookpath.InRoot(merged, shell, env)
	if err != nil {
		return 1, err
	}

	// Open the pty before the child chroots, while /dev/ptmx is reachable; the fds
	// remain valid across the chroot.
	var ptyMaster, ptySlave *os.File

	if tty {
		ptyMaster, ptySlave, err = openPty()
		if err != nil {
			return 1, fmt.Errorf("open pty: %w", err)
		}
	}

	// The chroot into merged is applied in the CHILD via SysProcAttr.Chroot — never in
	// this process, whose root must stay untouched.
	cmd := &exec.Cmd{
		Path: execPath,
		Args: args,
		Env:  env,
		Dir:  "/",
	}
	notifyCh := make(chan reaper.ProcessInfo, 8)

	usingReaper := reaper.Notify(notifyCh)
	if usingReaper {
		defer reaper.Stop(notifyCh)
	}

	if tty {
		return runWithOpenPty(ctx, cmd, merged, ptyMaster, ptySlave, stdin, stdout, cgroupFd, controlC, usingReaper, notifyCh)
	}

	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot:      merged,
		Setsid:      true,
		UseCgroupFD: true,
		CgroupFD:    int(cgroupFd.Fd()),
	}

	if startErr := cmd.Start(); startErr != nil {
		return 1, fmt.Errorf("start shell: %w", startErr)
	}

	setDebugOOMScore(cmd.Process.Pid)

	done := make(chan struct{})
	defer close(done)

	go runControlLoop(ctx, done, controlC, cmd.Process.Pid, nil)

	return waitHostNsCommand(cmd, usingReaper, notifyCh)
}

func waitHostNsCommand(cmd *exec.Cmd, usingReaper bool, notifyCh <-chan reaper.ProcessInfo) (int, error) {
	waitErr := reaper.WaitWrapper(usingReaper, notifyCh, cmd)
	if waitErr == nil {
		return 0, nil
	}

	if execErr, execErrOk := errors.AsType[*exec.ExitError](waitErr); execErrOk {
		return execErr.ExitCode(), nil
	}

	if reaperExitErr, reaperExitErrOk := errors.AsType[*reaper.ExitError](waitErr); reaperExitErrOk {
		return reaperExitErr.ExitCode, nil
	}

	return 1, waitErr
}

// runControlLoop handles out-of-band control messages for a running host-ns child:
//   - Signal: delivered to the child's process group so the shell and its
//     children all receive it (e.g., Ctrl-C kills the foreground pipeline).
//   - TermResize: applies TIOCSWINSZ on the pty master so the child's terminal
//     geometry stays in sync with the user's window.
//
// It also owns the child's lifetime relative to the stream: if ctx is canceled
// (client disconnect or transport error) it SIGKILLs the whole process group so
// the shell and its children don't leak as orphans.
//
// Exits when done is closed or ctx is canceled.
func runControlLoop(ctx context.Context, done <-chan struct{}, controlC <-chan hostNsControl, pid int, ptyMaster *os.File) {
	for {
		select {
		case <-done:
			return

		case <-ctx.Done():
			// Negative pid targets the process group (the child is a session
			// leader via Setsid), so pipelines and subshells die with the shell.
			syscall.Kill(-pid, syscall.SIGKILL) //nolint:errcheck

			return

		case ctrl := <-controlC:
			if ctrl.signal != 0 {
				// Negative pid targets the entire process group, ensuring that
				// child processes of the shell (pipelines, subshells) also receive
				// the signal rather than only the shell itself.
				syscall.Kill(-pid, syscall.Signal(ctrl.signal)) //nolint:errcheck
			}

			if ctrl.resize != nil && ptyMaster != nil {
				unix.IoctlSetWinsize(int(ptyMaster.Fd()), unix.TIOCSWINSZ, &unix.Winsize{ //nolint:errcheck
					Row: uint16(ctrl.resize.Height),
					Col: uint16(ctrl.resize.Width),
				})
			}
		}
	}
}

// setDebugOOMScore marks the given pid as more expendable than system services
// so that OOM kills target debug sessions before daemons. /proc is bind-mounted
// from the host so this write is valid inside the chroot.
func setDebugOOMScore(pid int) {
	path := fmt.Sprintf("/proc/%d/oom_score_adj", pid)
	if err := os.WriteFile(path, []byte("500"), 0o644); err != nil {
		log.Printf("host-ns: set oom_score_adj for pid %d: %v", pid, err)
	}
}

// runWithOpenPty starts cmd attached to already-opened pty fds and relays I/O
// between the pty master and the provided stdin/stdout streams.
// Both ptyMaster and ptySlave must have been opened before any chroot call.
func runWithOpenPty(
	ctx context.Context,
	cmd *exec.Cmd,
	chrootDir string,
	ptyMaster, ptySlave *os.File,
	stdin io.Reader,
	stdout io.Writer,
	cgroupFd *os.File,
	controlC <-chan hostNsControl,
	usingReaper bool,
	notifyCh <-chan reaper.ProcessInfo,
) (int, error) {
	defer ptyMaster.Close() //nolint:errcheck

	cmd.Stdin = ptySlave
	cmd.Stdout = ptySlave
	cmd.Stderr = ptySlave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot:      chrootDir,
		Setsid:      true,
		Setctty:     true,
		Ctty:        0, // fd 0 in the child (ptySlave as Stdin) is the controlling terminal
		UseCgroupFD: true,
		CgroupFD:    int(cgroupFd.Fd()),
	}

	if err := cmd.Start(); err != nil {
		ptySlave.Close() //nolint:errcheck

		return 1, fmt.Errorf("start shell (tty): %w", err)
	}

	ptySlave.Close() //nolint:errcheck

	setDebugOOMScore(cmd.Process.Pid)

	done := make(chan struct{})
	defer close(done)

	go runControlLoop(ctx, done, controlC, cmd.Process.Pid, ptyMaster)

	// Relay pty master → gRPC stdout.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, readErr := ptyMaster.Read(buf)
			if n > 0 {
				stdout.Write(buf[:n]) //nolint:errcheck
			}

			if readErr != nil {
				return
			}
		}
	}()

	// Relay gRPC stdin → pty master.
	go func() {
		buf := make([]byte, 1024)
		for {
			n, readErr := stdin.Read(buf)
			if n > 0 {
				ptyMaster.Write(buf[:n]) //nolint:errcheck
			}

			if readErr != nil {
				return
			}
		}
	}()

	return waitHostNsCommand(cmd, usingReaper, notifyCh)
}

// openPty opens a new pseudo-terminal pair using Linux's /dev/ptmx.
// Returns (master, slave) file handles.
func openPty() (*os.File, *os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open /dev/ptmx: %w", err)
	}

	// Unlock the slave pty. TIOCSPTLCK expects a pointer to int, not an inline value.
	if err = unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		master.Close() //nolint:errcheck

		return nil, nil, fmt.Errorf("unlock pty slave: %w", err)
	}

	// Get the slave pty index (TIOCGPTN) and construct its path.
	n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		master.Close() //nolint:errcheck

		return nil, nil, fmt.Errorf("get pty number: %w", err)
	}

	slaveName := fmt.Sprintf("/dev/pts/%d", n)

	slave, err := os.OpenFile(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close() //nolint:errcheck

		return nil, nil, fmt.Errorf("open slave pty %s: %w", slaveName, err)
	}

	return master, slave, nil
}
