// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

package debug

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/errdefs"
	ocispec "github.com/opencontainers/image-spec/identity"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// runHostNsContainer handles PROFILE_HOST_NS: it forks the host mount namespace,
// creates an overlay root (host / as lower + tmpfs upper), bind-mounts the image's
// /nix tree into the overlay so Nix store paths resolve, and execs the shell.
// Host binaries at their native paths work without nsenter; Nix tools are on PATH.
func runHostNsContainer(
	ctx context.Context,
	detachedCtx context.Context,
	c8dClient *containerd.Client,
	img containerd.Image,
	spec *machine.DebugContainerRunRequestSpec,
	srv grpc.BidiStreamingServer[machine.DebugContainerRunRequest, machine.DebugContainerRunResponse],
	containerID string,
	cgroupPath string,
) error {
	// 1. Create a writable snapshot of the image via Prepare (not View).
	//
	// Prepare places the writable upper layer in containerd's own data directory,
	// which is disk-backed on Talos. This means nix-env installs and other writes
	// to /nix go to disk instead of pinning RAM in a tmpfs upper layer.
	// The snapshot is removed by the deferred Remove below.
	snapshotKey := containerID + "-hostns-rw"

	diffIDs, err := img.RootFS(ctx)
	if err != nil {
		return fmt.Errorf("host-ns: get image rootfs: %w", err)
	}

	chainID := ocispec.ChainID(diffIDs).String()

	snapshotMounts, err := c8dClient.SnapshotService("").Prepare(ctx, snapshotKey, chainID)
	if err != nil {
		return fmt.Errorf("host-ns: prepare writable snapshot: %w", err)
	}

	defer func() {
		if rmErr := c8dClient.SnapshotService("").Remove(detachedCtx, snapshotKey); rmErr != nil && !errdefs.IsNotFound(rmErr) {
			log.Printf("host-ns: failed to remove snapshot %s: %v", snapshotKey, rmErr)
		}
	}()

	// 2. Working directory for overlay/image mounts — lives in /run (writable tmpfs).
	baseDir := filepath.Join("/run", "talos-debug-hostns-"+containerID)
	if err = os.MkdirAll(baseDir, 0o700); err != nil {
		return fmt.Errorf("host-ns: create work dir: %w", err)
	}

	defer os.RemoveAll(baseDir) //nolint:errcheck

	// 3. Wire up gRPC I/O streams.
	grpcStreamer, stdinR, stdoutW := newGrpcStreamWriter(srv)

	// 4. Determine command and args, matching PROFILE_PRIVILEGED semantics:
	// if spec.Args is non-empty it is the full argv (first element = executable);
	// otherwise default to the Nix bash.
	const defaultShell = "/nix/var/nix/profiles/default/bin/bash"

	var shell string
	var cmdArgs []string

	if args := spec.GetArgs(); len(args) > 0 {
		shell = args[0]
		cmdArgs = args
	} else {
		shell = defaultShell
		cmdArgs = []string{defaultShell}
	}

	// 5. Build env: Nix profile on PATH, preserve caller-supplied env overrides.
	// /root/.nix-profile/bin is where nix-env installs per-user packages;
	// it must come before the default profile so newly installed tools are found.
	userNixBin := "/root/.nix-profile/bin"
	nixBin := "/nix/var/nix/profiles/default/bin"
	nixSbin := "/nix/var/nix/profiles/default/sbin"
	hostPath := "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	env := []string{
		"PATH=" + userNixBin + ":" + nixBin + ":" + nixSbin + ":" + hostPath,
		"NIX_SSL_CERT_FILE=/nix/var/nix/profiles/default/etc/ssl/certs/ca-bundle.crt",
		"TERM=xterm-256color",
		"HOME=/root",
	}

	for k, v := range spec.GetEnv() {
		env = append(env, k+"="+v)
	}

	// 6. Open the per-session cgroup directory as an fd.
	// Passing this to SysProcAttr.CgroupFD places the child atomically into the
	// cgroup at fork time, avoiding the race of a post-fork cgroup.procs write.
	// Opened here (before the locked goroutine) while the path is still in the
	// original mount namespace; the fd stays valid across chroot.
	cgroupFd, err := os.Open(filepath.Join("/sys/fs/cgroup", cgroupPath))
	if err != nil {
		return fmt.Errorf("host-ns: open cgroup dir %s: %w", cgroupPath, err)
	}

	// 7. Control channel: carries signals and pty-resize events from the gRPC
	// recv loop into the locked goroutine that owns the child process and pty.
	// Buffered so the recv loop never blocks on a slow control handler.
	controlC := make(chan hostNsControl, 16)

	// 8. Launch the namespace-surgery goroutine.
	//
	// runtime.LockOSThread pins this goroutine to its OS thread.
	// All namespace calls (unshare, mount, chroot) apply to that thread only.
	// We deliberately never call UnlockOSThread: when the goroutine exits, Go
	// discards the tainted thread rather than returning it to the pool.
	exitC := make(chan int, 1)

	go func() {
		runtime.LockOSThread()

		code, launchErr := launchInHostNs(snapshotMounts, baseDir, shell, cmdArgs, env, stdinR, stdoutW, spec.GetTty(), cgroupFd, controlC)
		cgroupFd.Close() //nolint:errcheck

		if launchErr != nil {
			log.Printf("host-ns: launch error: %v", launchErr)
		}

		// Close the stdout pipe so the send loop drains and the streaming
		// coordinator receives EOF before we send the exit code.
		grpcStreamer.stdoutW.Close() //nolint:errcheck

		exitC <- code
	}()

	return grpcStreamer.streamHostNs(ctx, exitC, controlC)
}

// launchInHostNs performs the namespace surgery and runs the child process.
// Must be called from a goroutine that has called runtime.LockOSThread.
//
// Sequence:
//  1. unshare(CLONE_NEWNS)        — fork a private copy of the host mount namespace
//  2. MS_SLAVE|MS_REC on /        — no new mounts propagate back to the host
//  3. mount image snapshot         — image rootfs at baseDir/image
//  4. mount overlayfs              — host / as lower, tmpfs upper → merged at baseDir/merged
//  5. bind-mount image/nix          — writable because the snapshot was Prepare'd, not View'd;
//                                    nix-env writes go into containerd's disk-backed upper layer.
//  6. bind-mount /dev, /proc, /sys — expose host kernel filesystems in the overlay
//     (overlayfs only sees the squashfs lower dir, not the devtmpfs/procfs mounts)
//  8. open pty (if tty)            — must happen before chroot while /dev/ptmx is reachable
//  9. chroot into merged           — child sees host rootfs + /nix from image
// 10. exec shell
func launchInHostNs(
	snapshotMounts []mount.Mount,
	baseDir string,
	shell string,
	args []string,
	env []string,
	stdin io.Reader,
	stdout io.Writer,
	tty bool,
	cgroupFd *os.File,
	controlC <-chan hostNsControl,
) (exitCode int, err error) {
	imageDir := filepath.Join(baseDir, "image")
	upper := filepath.Join(baseDir, "upper")
	work := filepath.Join(baseDir, "work")
	merged := filepath.Join(baseDir, "merged")

	for _, d := range []string{imageDir, upper, work, merged} {
		if mkErr := os.MkdirAll(d, 0o755); mkErr != nil {
			return 1, fmt.Errorf("mkdir %s: %w", d, mkErr)
		}
	}

	// Fork the host mount namespace.
	if err = unix.Unshare(unix.CLONE_NEWNS); err != nil {
		return 1, fmt.Errorf("unshare CLONE_NEWNS: %w", err)
	}

	// Prevent new mounts from propagating back to the host namespace.
	if err = unix.Mount("", "/", "", unix.MS_SLAVE|unix.MS_REC, ""); err != nil {
		return 1, fmt.Errorf("set mount propagation to slave: %w", err)
	}

	// Mount the image snapshot (overlayfs layers from containerd) at imageDir.
	if err = mount.All(snapshotMounts, imageDir); err != nil {
		return 1, fmt.Errorf("mount image snapshot: %w", err)
	}

	defer unix.Unmount(imageDir, unix.MNT_DETACH) //nolint:errcheck

	// Create overlay: lower=host root, upper=tmpfs dir, merged=new chroot root.
	// The upper layer allows creating /nix without touching Talos's immutable squashfs.
	overlayOpts := fmt.Sprintf("lowerdir=/,upperdir=%s,workdir=%s", upper, work)
	if err = unix.Mount("overlay", merged, "overlay", 0, overlayOpts); err != nil {
		return 1, fmt.Errorf("mount overlay root: %w", err)
	}

	defer unix.Unmount(merged, unix.MNT_DETACH) //nolint:errcheck

	// Bind-mount the image's /nix into the overlay root.
	// The snapshot was Prepare'd (writable), so its upper layer lives in
	// containerd's disk-backed data directory. nix-env installs write there
	// instead of pinning RAM in a tmpfs upper layer.
	nixSrc := filepath.Join(imageDir, "nix")
	nixDst := filepath.Join(merged, "nix")

	if mkErr := os.MkdirAll(nixDst, 0o755); mkErr != nil {
		return 1, fmt.Errorf("mkdir /nix in overlay: %w", mkErr)
	}

	if err = unix.Mount(nixSrc, nixDst, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return 1, fmt.Errorf("bind-mount /nix: %w", err)
	}

	// The overlayfs lowerdir is the raw squashfs filesystem, not the VFS mount tree.
	// This means /dev (devtmpfs), /proc (procfs), and /sys (sysfs) are NOT inherited
	// through the overlay — only their empty squashfs mount-point directories are visible.
	// Bind-mount the host's live versions in so the child and its tools work correctly.
	for _, dir := range []string{"dev", "proc", "sys"} {
		src := "/" + dir
		dst := filepath.Join(merged, dir)

		if mkErr := os.MkdirAll(dst, 0o755); mkErr != nil {
			return 1, fmt.Errorf("mkdir %s in overlay: %w", dir, mkErr)
		}

		if mntErr := unix.Mount(src, dst, "", unix.MS_BIND|unix.MS_REC, ""); mntErr != nil {
			return 1, fmt.Errorf("bind-mount /%s into overlay: %w", dir, mntErr)
		}
	}

	// Copy the host's live /etc/resolv.conf into the overlay.
	// The overlay's lowerdir is the raw squashfs which doesn't carry Talos's
	// runtime bind-mount of resolv.conf, so DNS would be broken without this.
	if hostResolv, readErr := os.ReadFile("/etc/resolv.conf"); readErr == nil && len(hostResolv) > 0 {
		resolvDst := filepath.Join(merged, "etc", "resolv.conf")

		if mkErr := os.MkdirAll(filepath.Dir(resolvDst), 0o755); mkErr == nil {
			os.WriteFile(resolvDst, hostResolv, 0o644) //nolint:errcheck
		}
	}

	// Write /etc/nix/nix.conf before chroot so the nix package manager works
	// without requiring the nixbld build-users group (which doesn't exist in
	// the chroot) and without needing a sandbox infrastructure.
	nixConfDst := filepath.Join(merged, "etc", "nix", "nix.conf")
	if mkErr := os.MkdirAll(filepath.Dir(nixConfDst), 0o755); mkErr == nil {
		os.WriteFile(nixConfDst, []byte( //nolint:errcheck
			"build-users-group =\n"+
				"sandbox = false\n"+
				"experimental-features = nix-command flakes\n",
		), 0o644)
	}

	// Open the pty HERE, before chroot, while /dev/ptmx is reachable from the
	// host's devtmpfs. The fds remain valid across chroot.
	var ptyMaster, ptySlave *os.File

	if tty {
		ptyMaster, ptySlave, err = openPty()
		if err != nil {
			return 1, fmt.Errorf("open pty: %w", err)
		}
	}

	// chroot into the overlay.
	// After this point all absolute paths resolve from the merged root:
	//   /usr/local/sbin/zpool → host binary  ✓
	//   /nix/store/<h>/bin/jq → image tool   ✓
	if err = unix.Chroot(merged); err != nil {
		if ptyMaster != nil {
			ptyMaster.Close() //nolint:errcheck
		}

		if ptySlave != nil {
			ptySlave.Close() //nolint:errcheck
		}

		return 1, fmt.Errorf("chroot into overlay: %w", err)
	}

	if err = os.Chdir("/"); err != nil {
		return 1, fmt.Errorf("chdir /: %w", err)
	}

	// Build and start the child process.
	// The child inherits our private, forked mount namespace (with /nix from the image).
	cmd := &exec.Cmd{
		Path: shell,
		Args: args,
		Env:  env,
	}

	if tty {
		return runWithOpenPty(cmd, ptyMaster, ptySlave, stdin, stdout, cgroupFd, controlC)
	}

	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	cmd.SysProcAttr = &syscall.SysProcAttr{
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

	go runControlLoop(done, controlC, cmd.Process.Pid, nil)

	if waitErr := cmd.Wait(); waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}

		return 1, waitErr
	}

	return 0, nil
}

// runControlLoop handles out-of-band control messages for a running host-ns child:
//   - Signal: delivered to the child's process group so the shell and its
//     children all receive it (e.g., Ctrl-C kills the foreground pipeline).
//   - TermResize: applies TIOCSWINSZ on the pty master so the child's terminal
//     geometry stays in sync with the user's window.
//
// Runs in a plain goroutine (no LockOSThread needed — Kill and IoctlSetWinsize
// are thread-agnostic syscalls). Exits when done is closed.
func runControlLoop(done <-chan struct{}, controlC <-chan hostNsControl, pid int, ptyMaster *os.File) {
	for {
		select {
		case <-done:
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
func runWithOpenPty(cmd *exec.Cmd, ptyMaster, ptySlave *os.File, stdin io.Reader, stdout io.Writer, cgroupFd *os.File, controlC <-chan hostNsControl) (int, error) {
	defer ptyMaster.Close() //nolint:errcheck

	cmd.Stdin = ptySlave
	cmd.Stdout = ptySlave
	cmd.Stderr = ptySlave
	cmd.SysProcAttr = &syscall.SysProcAttr{
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

	go runControlLoop(done, controlC, cmd.Process.Pid, ptyMaster)

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

	if waitErr := cmd.Wait(); waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}

		return 1, waitErr
	}

	return 0, nil
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
