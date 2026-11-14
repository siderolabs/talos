// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package sandboxd manages the sandbox PID+mount namespace in which machined's
// container-plane services (cri, kubelet, pods) run, isolated from machined.
package sandboxd

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/siderolabs/go-cmd/pkg/cmd/proc/reaper"
	"golang.org/x/sys/unix"
	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/sandboxd/internal/protocol"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	// readyFD is the pipe sandboxd writes one byte to when ready.
	readyFD = 3
	// controlFD is the SEQPACKET socket machined sends launch requests on.
	controlFD = 4

	// oomScoreAdjInit is the oom_score_adj applied to sandboxd. It is PID 1
	// of the sandbox PID namespace: if the OOM killer reaps it, the kernel tears
	// down the whole namespace (cri, kubelet and every pod). It must therefore be a
	// last-resort victim; -1000 disables OOM killing for it. Services get their own
	// (higher) oom_score_adj applied by machined after launch.
	oomScoreAdjInit = -1000
)

// logf writes an operational log line to sandboxd's stderr. machined wires
// sandboxd's stdout/stderr to the logging manager, so these lines show up as the
// "sandboxd" service log (talosctl logs sandboxd).
func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "sandboxd: "+format+"\n", args...)
}

// setSelfOOMScoreAdj sets the calling process's oom_score_adj. Lowering it below
// 0 requires CAP_SYS_RESOURCE, which machined and its children hold.
func setSelfOOMScoreAdj(adj int) {
	if err := os.WriteFile("/proc/self/oom_score_adj", []byte(strconv.Itoa(adj)), 0); err != nil {
		logf("failed to set oom_score_adj=%d: %v", adj, err)
	}
}

func ignoreTerminationSignals() func() {
	// Catch rather than SIG_IGN: exec resets caught signals to their defaults,
	// whereas ignored dispositions would leak into services and their children.
	// Notify installs the handlers synchronously and never blocks on delivery.
	// Intentionally leave the channel unconsumed to discard every notification.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)

	return func() { signal.Stop(signals) }
}

// Main is the entry point for the sandboxd subprocess. It is PID 1 of
// the sandbox PID+mount namespace and:
//  1. protects itself from the OOM killer and scopes /proc to this PID namespace,
//  2. catches and discards termination signals and reaps orphaned children,
//  3. serves launch requests from machined, forking each service directly into
//     this namespace (no setns).
func Main() {
	// Protect PID 1 of the sandbox namespace from the OOM killer before anything
	// else: its death collapses the entire namespace.
	setSelfOOMScoreAdj(oomScoreAdjInit)

	// Replace the inherited host /proc with one scoped to THIS PID namespace.
	//
	// The host /proc is detached rather than overmounted: an overmounted host /proc
	// stays in this namespace, and unmounting the scoped /proc on top exposes the
	// host PID namespace (machined's fds and detached mounts).
	//
	// Mount and unmount events propagate through the parent mount, and every mount
	// inherited from machined's namespace (/, /run, /system, /dev, ...) is a peer of
	// machined's copy: make them all receive-only first, or detaching /proc here
	// detaches machined's /proc too, and pod mounts under /run reach machined.
	// /proc is made private so its submounts do not propagate either.
	//
	// The tree is then shared again, in peer groups of its own: mounts machined makes
	// later (e.g. /var) arrive shared within the sandbox, so kubelet's volume mounts
	// still reach CRI and the pods.
	if err := syscall.Mount("", "/", "", syscall.MS_SLAVE|syscall.MS_REC, ""); err != nil {
		logf("make / a slave: %v", err)
		os.Exit(1)
	}

	if err := syscall.Mount("", "/", "", syscall.MS_SHARED|syscall.MS_REC, ""); err != nil {
		logf("make / shared: %v", err)
		os.Exit(1)
	}

	if err := syscall.Mount("", "/proc", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		logf("make /proc private: %v", err)
		os.Exit(1)
	}

	if err := syscall.Unmount("/proc", syscall.MNT_DETACH); err != nil {
		logf("detach host /proc: %v", err)
		os.Exit(1)
	}

	if err := syscall.Mount("proc", "/proc", "proc",
		syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, ""); err != nil {
		logf("remount /proc: %v", err)
		os.Exit(1)
	}

	// Never let the control socket leak into a forked service.
	syscall.CloseOnExec(controlFD)

	// Wrap the control socket before starting the reaper, so a failure here can
	// os.Exit without skipping a deferred reaper.Shutdown().
	controlConn, err := protocol.UnixConn(os.NewFile(controlFD, "sandboxd-control"))
	if err != nil {
		logf("wrap control socket: %v", err)
		os.Exit(1)
	}

	// As PID 1, catch and discard TERM/INT/HUP/QUIT so in-namespace `kill 1`
	// is a no-op without passing SIG_IGN to children. The kernel already blocks
	// namespace-local SIGKILL/SIGSTOP; machined can still tear us down via SIGKILL.
	// Install handlers before launching any children and release them on exit.
	stopSignals := ignoreTerminationSignals()
	defer stopSignals()

	// Reap orphaned children (e.g. containerd shims left behind when CRI is killed)
	// and the services this init forks.
	reaper.Run()

	defer reaper.Shutdown()

	// Signal machined that the namespace is up and the control socket is served.
	ready := os.NewFile(readyFD, "ready")
	ready.Write([]byte{1}) //nolint:errcheck
	ready.Close()          //nolint:errcheck

	logf("started as PID %d of the sandbox namespace; serving launch requests", os.Getpid())

	// Serve requests until machined closes the control socket (shutdown).
	for {
		req, fds, err := recvRequest(controlConn)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				logf("control socket closed; shutting down")

				return
			}

			logf("receiving request: %v", err)

			continue
		}

		if req.Launch != nil {
			go handleLaunch(*req.Launch, [4]*os.File(fds))
		} else {
			go handleSetattr(*req.Setattr, fds[0])
		}
	}
}

// recvRequest reads one request datagram plus its SCM_RIGHTS fds from the
// control socket.
func recvRequest(control *net.UnixConn) (protocol.Request, []*os.File, error) {
	var req protocol.Request

	buf := make([]byte, 64*1024)                // request holds args+env; keep the SEQPACKET datagram whole
	oob := make([]byte, syscall.CmsgSpace(4*4)) // room for 4 fds

	n, oobn, _, _, err := control.ReadMsgUnix(buf, oob)
	if err != nil {
		return req, nil, err
	}

	if n == 0 {
		return req, nil, io.EOF
	}

	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return req, nil, fmt.Errorf("parsing control message: %w", err)
	}

	var rawFds []int

	for i := range scms {
		fdSet, perr := syscall.ParseUnixRights(&scms[i])
		if perr != nil {
			for _, fd := range rawFds {
				syscall.Close(fd) //nolint:errcheck
			}

			return req, nil, fmt.Errorf("parsing SCM_RIGHTS: %w", perr)
		}

		rawFds = append(rawFds, fdSet...)
	}

	fds := make([]*os.File, 0, len(rawFds))

	// ReadMsgUnix receives the fds close-on-exec (MSG_CMSG_CLOEXEC): only the
	// explicit stdio (re-established via pa.Files) reaches the forked service.
	for _, fd := range rawFds {
		fds = append(fds, os.NewFile(uintptr(fd), "request-fd"))
	}

	req, err = protocol.DecodeRequest(buf[:n], len(fds))
	if err != nil {
		for _, f := range fds {
			f.Close() //nolint:errcheck
		}

		return req, nil, err
	}

	return req, fds, nil
}

// handleSetattr changes the attributes of the mount at the target in this namespace.
func handleSetattr(req protocol.SetattrRequest, reply *os.File) {
	if err := protocol.ServeSetattr(reply, req, func(req protocol.SetattrRequest) error {
		// The target must be the root of a mount, or mount_setattr fails with EINVAL:
		// it never changes the mount containing the target.
		return unix.MountSetattr(unix.AT_FDCWD, req.Target, unix.AT_SYMLINK_NOFOLLOW, &unix.MountAttr{
			Attr_set: req.Set,
			Attr_clr: req.Clr,
		})
	}); err != nil {
		logf("answer setattr: %v", err)
	}
}

// handleLaunch forks one service into the sandbox namespace and relays its
// pidfd and exit status back to machined over the per-service socket.
//
//nolint:gocyclo
func handleLaunch(req protocol.LaunchRequest, fds [4]*os.File) {
	stdin, stdout, stderr, svcFile := fds[0], fds[1], fds[2], fds[3]

	defer stdin.Close()  //nolint:errcheck
	defer stdout.Close() //nolint:errcheck
	defer stderr.Close() //nolint:errcheck

	svcConn, err := protocol.UnixConn(svcFile)
	if err != nil {
		logf("wrap service socket: %v", err)

		return
	}

	defer svcConn.Close() //nolint:errcheck

	launcher := cap.NewLauncher(req.Args[0], req.Args, req.Env)
	launcher.Callback(beforeServiceExec)

	if err := dropCaps(req.DroppedCapabilities, launcher); err != nil {
		sendLaunchErr(svcConn, fmt.Sprintf("drop capabilities for %s: %v", req.Args[0], err))

		return
	}

	notifyCh := make(chan reaper.ProcessInfo, 8)
	reaper.Notify(notifyCh)

	defer reaper.Stop(notifyCh)

	pid, startErr := launcher.Launch(&launchContext{
		selinuxLabel: req.SelinuxLabel,
		stdin:        stdin,
		stdout:       stdout,
		stderr:       stderr,
	})
	if startErr != nil {
		sendLaunchErr(svcConn, fmt.Sprintf("exec %s: %v", req.Args[0], startErr))

		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		sendLaunchErr(svcConn, fmt.Sprintf("find process for %s: %v", req.Args[0], err))

		return
	}

	pidFd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		process.Kill()                                     //nolint:errcheck
		reaper.ProcessWaitWrapper(true, notifyCh, process) //nolint:errcheck
		sendLaunchErr(svcConn, fmt.Sprintf("open pidfd for %s: %v", req.Args[0], err))

		return
	}

	defer syscall.Close(pidFd) //nolint:errcheck

	// Start awaiting exit now (the reaper reaps our children), so an exit that
	// happens before machined calls Wait is captured.
	exitCh := make(chan error, 1)
	go func() {
		exitCh <- reaper.ProcessWaitWrapper(true, notifyCh, process)
	}()

	// Return the pidfd; machined resolves the host PID and signals through it.
	var respBuf bytes.Buffer
	if encErr := gob.NewEncoder(&respBuf).Encode(protocol.LaunchResponse{}); encErr != nil {
		logf("encode launch response: %v", encErr)
	}

	if _, _, err := svcConn.WriteMsgUnix(respBuf.Bytes(), syscall.UnixRights(pidFd), nil); err != nil {
		process.Kill() //nolint:errcheck
		<-exitCh

		return
	}

	logf("launched %s [pid %d]", req.Args[0], pid)

	// Wait for machined to ask (1 byte), then report the exit. If machined closes
	// the socket without asking, kill the service.
	var signalBuf [1]byte

	if _, err := svcConn.Read(signalBuf[:]); err != nil {
		process.Kill() //nolint:errcheck
		<-exitCh

		return
	}

	waitErr := <-exitCh

	resp := protocol.WaitResponse{}

	if waitErr != nil {
		// The caller only uses the error; a non-zero/failed exit is reported via
		// ExitErr (ProcessWaitWrapper does not surface the numeric code).
		resp.ExitCode = -1
		resp.ExitErr = waitErr.Error()

		logf("%s [pid %d] exited: %v", req.Args[0], pid, waitErr)
	} else {
		logf("%s [pid %d] exited cleanly", req.Args[0], pid)
	}

	var waitBuf bytes.Buffer
	if encErr := gob.NewEncoder(&waitBuf).Encode(resp); encErr != nil {
		logf("encode wait response: %v", encErr)
	}

	svcConn.Write(waitBuf.Bytes()) //nolint:errcheck
}

// launchContext is passed to the libcap launch callback.
type launchContext struct {
	selinuxLabel string
	stdin        *os.File
	stdout       *os.File
	stderr       *os.File
}

// beforeServiceExec runs on libcap's launch thread just before it forks+execs
// the service. The init is already inside the sandbox namespace, so the child
// is created there directly (no setns).
func beforeServiceExec(pa *syscall.ProcAttr, data any) error {
	cfg, ok := data.(*launchContext)
	if !ok {
		return fmt.Errorf("failed to get sandbox launch context")
	}

	pa.Files = []uintptr{cfg.stdin.Fd(), cfg.stdout.Fd(), cfg.stderr.Fd()}

	if selinux.IsEnabled() {
		label := cfg.selinuxLabel
		if label == "" {
			label = constants.SelinuxLabelUnconfinedService
		}

		// The init is PID 1 of this namespace, so /proc/thread-self resolves in the
		// scoped /proc; write the pending exec label directly. It is consumed by the
		// execve, transitioning the service to its domain (e.g. pod_containerd_t).
		execAttr, err := os.OpenFile("/proc/thread-self/attr/exec", os.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("opening SELinux exec label attr: %w", err)
		}

		defer execAttr.Close() //nolint:errcheck

		if err := writeFull(execAttr, []byte(label)); err != nil {
			return fmt.Errorf("setting SELinux exec label %q: %w", label, err)
		}
	}

	// Mark every fd >= 3 close-on-exec so the service inherits only stdin/stdout/
	// stderr (re-established via pa.Files above); the control socket, this launch's
	// service socket, other in-flight launches' fds and Go-runtime fds must not
	// leak into the service.
	if err := unix.CloseRange(3, math.MaxUint32, unix.CLOSE_RANGE_CLOEXEC); err != nil {
		return fmt.Errorf("close_range(CLOEXEC): %w", err)
	}

	return nil
}

func dropCaps(droppedCapabilities []string, launcher *cap.Launcher) error {
	dropped := make([]cap.Value, 0, len(droppedCapabilities))

	for _, name := range droppedCapabilities {
		capability, err := cap.FromName(name)
		if err != nil {
			return fmt.Errorf("parse capability %q: %w", name, err)
		}

		dropped = append(dropped, capability)
	}

	iab := cap.IABGetProc()
	if err := iab.SetVector(cap.Bound, true, dropped...); err != nil {
		return fmt.Errorf("set capability bounding set: %w", err)
	}

	launcher.SetIAB(iab)

	return nil
}

func sendLaunchErr(svcConn *net.UnixConn, errMsg string) {
	var buf bytes.Buffer

	if err := gob.NewEncoder(&buf).Encode(protocol.LaunchResponse{Err: errMsg}); err != nil {
		return
	}

	svcConn.Write(buf.Bytes()) //nolint:errcheck
}
