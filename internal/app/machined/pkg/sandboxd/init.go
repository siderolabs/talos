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

	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Control protocol between machined (Client) and sandboxd.
//
// machined sends a launchRequest over the control socket as one SEQPACKET
// datagram, with SCM_RIGHTS carrying [stdin, stdout, stderr, serviceSocket].
// sandboxd - which is PID 1 inside the namespace - forks the service
// directly (no setns needed) and uses serviceSocket to (1) return a
// launchResponse plus the service pidfd via SCM_RIGHTS, and (2) after machined
// asks (Wait), return a waitResponse carrying the exit status.

// launchRequest is sent by machined for each service launch.
type launchRequest struct {
	Args                []string
	Env                 []string
	DroppedCapabilities []string
	SelinuxLabel        string
}

// launchResponse is returned once the service is started; the service pidfd is
// passed via SCM_RIGHTS alongside it. On error (Err != ""), no pidfd is sent.
type launchResponse struct {
	Err string
}

// waitResponse is returned after the service exits.
type waitResponse struct {
	ExitCode int
	ExitErr  string
}

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

// Main is the entry point for the sandboxd subprocess. It is PID 1 of
// the sandbox PID+mount namespace and:
//  1. protects itself from the OOM killer and scopes /proc to this PID namespace,
//  2. ignores namespace-local termination signals and reaps orphaned children,
//  3. serves launch requests from machined, forking each service directly into
//     this namespace (no setns).
func Main() {
	// Protect PID 1 of the sandbox namespace from the OOM killer before anything
	// else: its death collapses the entire namespace.
	setSelfOOMScoreAdj(oomScoreAdjInit)

	// Make /proc private so the overmount below stays in this mount namespace,
	// then remount /proc scoped to THIS PID namespace.
	if err := syscall.Mount("", "/proc", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		logf("make /proc private: %v", err)
		os.Exit(1)
	}

	if err := syscall.Mount("proc", "/proc", "proc",
		syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, ""); err != nil {
		logf("remount /proc: %v", err)
		os.Exit(1)
	}

	// Never let the control socket leak into a forked service.
	syscall.CloseOnExec(controlFD)

	// As PID 1 of the sandbox namespace, ignore the catchable termination
	// signals. The kernel already blocks SIGKILL/SIGSTOP to a namespace init from
	// within its own namespace; ignoring TERM/INT/HUP/QUIT (which the Go runtime
	// would otherwise turn into an exit) makes in-namespace `kill 1` a no-op, so
	// only machined (the parent) can tear it down via SIGKILL.
	signal.Ignore(syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)

	// Wrap the control socket before starting the reaper, so a failure here can
	// os.Exit without skipping a deferred reaper.Shutdown().
	controlConn, err := unixConn(os.NewFile(controlFD, "sandboxd-control"))
	if err != nil {
		logf("wrap control socket: %v", err)
		os.Exit(1)
	}

	// Reap orphaned children (e.g. containerd shims left behind when CRI is killed)
	// and the services this init forks.
	reaper.Run()

	defer reaper.Shutdown()

	// Signal machined that the namespace is up and the control socket is served.
	ready := os.NewFile(readyFD, "ready")
	ready.Write([]byte{1}) //nolint:errcheck
	ready.Close()          //nolint:errcheck

	logf("started as PID %d of the sandbox namespace; serving launch requests", os.Getpid())

	// Serve launch requests until machined closes the control socket (shutdown).
	for {
		req, fds, err := recvLaunch(controlConn)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				logf("control socket closed; shutting down")

				return
			}

			logf("receiving launch request: %v", err)

			continue
		}

		go handleLaunch(req, fds)
	}
}

// recvLaunch reads one launch request datagram plus its SCM_RIGHTS fds
// [stdin, stdout, stderr, serviceSocket] from the control socket.
//
//nolint:gocyclo
func recvLaunch(control *net.UnixConn) (launchRequest, [4]*os.File, error) {
	var (
		req launchRequest
		fds [4]*os.File
	)

	buf := make([]byte, 64*1024)                // request holds args+env; keep the SEQPACKET datagram whole
	oob := make([]byte, syscall.CmsgSpace(4*4)) // room for 4 fds

	n, oobn, _, _, err := control.ReadMsgUnix(buf, oob)
	if err != nil {
		return req, fds, err
	}

	if n == 0 {
		return req, fds, io.EOF
	}

	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return req, fds, fmt.Errorf("parsing control message: %w", err)
	}

	var rawFds []int

	for i := range scms {
		fdSet, perr := syscall.ParseUnixRights(&scms[i])
		if perr != nil {
			return req, fds, fmt.Errorf("parsing SCM_RIGHTS: %w", perr)
		}

		rawFds = append(rawFds, fdSet...)
	}

	if len(rawFds) != len(fds) {
		for _, fd := range rawFds {
			syscall.Close(fd) //nolint:errcheck
		}

		return req, fds, fmt.Errorf("expected %d fds, got %d", len(fds), len(rawFds))
	}

	names := [4]string{"stdin", "stdout", "stderr", "service-socket"}

	for i, fd := range rawFds {
		// Received fds are not close-on-exec; mark them so nothing but the explicit
		// stdio (re-established via pa.Files) leaks into the forked service.
		syscall.CloseOnExec(fd)
		fds[i] = os.NewFile(uintptr(fd), names[i])
	}

	if err := gob.NewDecoder(bytes.NewReader(buf[:n])).Decode(&req); err != nil {
		for i := range fds {
			fds[i].Close() //nolint:errcheck
		}

		return req, fds, fmt.Errorf("decoding launch request: %w", err)
	}

	return req, fds, nil
}

// handleLaunch forks one service into the sandbox namespace and relays its
// pidfd and exit status back to machined over the per-service socket.
//
//nolint:gocyclo
func handleLaunch(req launchRequest, fds [4]*os.File) {
	stdin, stdout, stderr, svcFile := fds[0], fds[1], fds[2], fds[3]

	defer stdin.Close()  //nolint:errcheck
	defer stdout.Close() //nolint:errcheck
	defer stderr.Close() //nolint:errcheck

	svcConn, err := unixConn(svcFile)
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
	if encErr := gob.NewEncoder(&respBuf).Encode(launchResponse{}); encErr != nil {
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

	resp := waitResponse{}

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

	if err := gob.NewEncoder(&buf).Encode(launchResponse{Err: errMsg}); err != nil {
		return
	}

	svcConn.Write(buf.Bytes()) //nolint:errcheck
}

// unixConn wraps an *os.File socket as a *net.UnixConn. FileConn duplicates the
// fd, so the original file is closed.
func unixConn(f *os.File) (*net.UnixConn, error) {
	conn, err := net.FileConn(f)
	f.Close() //nolint:errcheck

	if err != nil {
		return nil, err
	}

	uc, ok := conn.(*net.UnixConn)
	if !ok {
		conn.Close() //nolint:errcheck

		return nil, fmt.Errorf("not a unix socket")
	}

	return uc, nil
}
