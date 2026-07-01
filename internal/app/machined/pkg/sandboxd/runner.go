// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sandboxd

import (
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/siderolabs/go-cmd/pkg/cmd/proc/reaper"
	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/pid"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// ServiceID is the ID of the system service that owns the sandbox
// PID+mount namespace.
const ServiceID = "sandboxd"

// sandboxRunner runs the sandboxd subprocess that anchors the sandbox
// PID+mount namespace: it spawns it (in a fresh PID+mount namespace), keeps the
// machined end of the control socket over which the init forks services,
// publishes a launcher to the runtime, and tears it down on exit.
//
// It is wrapped in restart.Forever by the service. sandboxd dying is
// turned by the kernel into the destruction of the entire namespace (cri,
// kubelet, all pods); restart recovers by recreating the namespace from scratch,
// and the dependent services (whose processes the kernel also killed) re-launch
// into the new namespace via the per-launch launcher getter.
type sandboxRunner struct {
	rt runtime.Runtime

	stop    chan struct{}
	stopped chan struct{}
}

// NewRunner returns a runner.Runner managing the sandboxd lifecycle.
func NewRunner(rt runtime.Runtime) runner.Runner {
	return &sandboxRunner{
		rt:      rt,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

// Open implements runner.Runner.
func (s *sandboxRunner) Open() error { return nil }

// Close implements runner.Runner.
func (s *sandboxRunner) Close() error { return nil }

// String implements runner.Runner.
func (s *sandboxRunner) String() string { return "Process(\"sandboxd\")" }

// Stop implements runner.Runner.
func (s *sandboxRunner) Stop() error {
	close(s.stop)

	<-s.stopped

	s.stop = make(chan struct{})
	s.stopped = make(chan struct{})

	return nil
}

// Run implements runner.Runner.
//
//nolint:gocyclo
func (s *sandboxRunner) Run(eventSink events.Recorder, pidRecorder pid.Recorder) error {
	defer close(s.stopped)

	readyR, readyW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("creating ready pipe: %w", err)
	}

	defer readyR.Close() //nolint:errcheck

	// sandboxd's stdin (/dev/null); it never reads stdin.
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return fmt.Errorf("opening /dev/null: %w", err)
	}

	defer devNull.Close() //nolint:errcheck

	// Route sandboxd's stdout/stderr through the logging manager, so it registers a
	// "sandboxd" log and `talosctl logs sandboxd` works like any other service.
	// machined reads logR; sandboxd writes logW (its fds 1 and 2).
	logSink, err := s.rt.Logging().ServiceLog(ServiceID).Writer()
	if err != nil {
		return fmt.Errorf("creating sandboxd log writer: %w", err)
	}

	logR, logW, err := os.Pipe()
	if err != nil {
		logSink.Close() //nolint:errcheck

		return fmt.Errorf("creating log pipe: %w", err)
	}

	defer logW.Close() //nolint:errcheck // child owns its copy after launch

	go func() {
		defer logR.Close()    //nolint:errcheck
		defer logSink.Close() //nolint:errcheck

		io.Copy(logSink, logR) //nolint:errcheck
	}()

	// Control socket: machined sends launch requests over it; sandboxd forks each
	// service and replies over the per-service socket carried in the request.
	// Created here, after the fallible setup above, so nothing between its creation
	// and the launch can fail — controlInit then needs only its post-launch close.
	controlFds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_SEQPACKET|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("creating control socketpair: %w", err)
	}

	controlMachined := os.NewFile(uintptr(controlFds[0]), "control-machined")
	controlInit := os.NewFile(uintptr(controlFds[1]), "control-init")

	// controlMachined is consumed by unixConn below (dups the fd, closes this
	// handle) on success, or dropped on any error path; this defer covers both.
	// (os.File.Close is idempotent, so the success-path double close is a no-op.)
	defer controlMachined.Close() //nolint:errcheck

	// Re-exec the machined binary under its "sandboxd" personality: /sbin/sandboxd
	// is a hardlink to /usr/bin/init, dispatched by argv0 basename in main (like
	// poweroff/shutdown/dashboard). Launch via libcap so the callback can set the
	// pending SELinux exec label on the fork thread (init_t -> sandboxd_t) without
	// contaminating machined's other threads — the mechanism the process runner
	// uses for dashboard/apid. fd 3 = ready pipe, fd 4 = control socket.
	launcher := cap.NewLauncher("/sbin/sandboxd", []string{"/sbin/sandboxd"}, os.Environ())
	launcher.Callback(beforeSandboxdExec)

	notifyCh := make(chan reaper.ProcessInfo, 8)

	usingReaper := reaper.Notify(notifyCh)
	if usingReaper {
		defer reaper.Stop(notifyCh)
	}

	pid, err := launcher.Launch(&sandboxdLaunchContext{
		files: []uintptr{devNull.Fd(), logW.Fd(), logW.Fd(), readyW.Fd(), controlInit.Fd()},
	})

	// The child holds its own copies of the passed fds now; drop ours. Closing
	// controlInit here is required (not just cleanup): only once machined stops
	// holding the child's control end does a read on controlMachined see EOF when
	// sandboxd dies. Closing readyW lets readyR see EOF if the child dies early.
	readyW.Close()      //nolint:errcheck
	controlInit.Close() //nolint:errcheck

	if err != nil {
		return fmt.Errorf("launching sandboxd: %w", err)
	}

	// os.FindProcess never fails on Linux (returns a process handle for any pid).
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("finding process for PID %d: %w", pid, err)
	}

	waitCh := make(chan error, 1)

	go func() {
		waitCh <- reaper.ProcessWaitWrapper(usingReaper, notifyCh, process)
	}()

	// Wait for the init to signal readiness (or die before then).
	var buf [1]byte

	if _, rerr := readyR.Read(buf[:]); rerr != nil {
		process.Kill() //nolint:errcheck
		<-waitCh

		return fmt.Errorf("sandboxd failed to start: %w", rerr)
	}

	controlConn, err := unixConn(controlMachined)
	if err != nil {
		process.Kill() //nolint:errcheck
		<-waitCh

		return fmt.Errorf("wrapping control socket: %w", err)
	}

	client := &Client{control: controlConn}

	// Publish the launcher; clear it again whenever this namespace goes away so
	// in-flight launches fail fast instead of entering a dead namespace.
	s.rt.SetSandbox(client)

	teardown := func() {
		s.rt.SetSandbox(nil)
		client.close()
	}

	if err := pidRecorder(ServiceID, int32(process.Pid), false); err != nil {
		process.Kill() //nolint:errcheck
		<-waitCh
		teardown()

		return fmt.Errorf("recording pid: %w", err)
	}

	defer pidRecorder(ServiceID, int32(process.Pid), true) //nolint:errcheck

	eventSink(events.StateRunning, "sandboxd started with PID %d", process.Pid)

	select {
	case werr := <-waitCh:
		// Unexpected exit: the kernel has torn down the sandbox namespace. Return
		// an error so restart.Forever recreates it.
		fmt.Fprintf(logW, "machined: sandboxd (pid %d) exited, sandbox namespace torn down; restarting\n", process.Pid) //nolint:errcheck

		teardown()

		if werr == nil {
			werr = fmt.Errorf("exited")
		}

		return fmt.Errorf("sandboxd exited, namespace lost: %w", werr)
	case <-s.stop:
		// sandboxd ignores SIGTERM (it is PID 1 of its namespace); SIGKILL it from
		// the parent namespace, which the kernel then turns into teardown of the
		// whole namespace.
		eventSink(events.StateStopping, "Sending SIGKILL to sandboxd")

		process.Kill() //nolint:errcheck
		<-waitCh
		teardown()

		return nil
	}
}

// sandboxdLaunchContext is passed to the libcap launch callback for sandboxd.
type sandboxdLaunchContext struct {
	files []uintptr // stdin, stdout, stderr, ready pipe, control socket
}

// beforeSandboxdExec runs on libcap's launch thread just before it forks+execs
// sandboxd. It wires sandboxd's fds, places it in a fresh PID+mount namespace,
// and (under SELinux) writes the pending exec label so the child transitions
// from init_t (machined) to sandboxd_t. libcap guarantees this runs on the
// thread that forks and that privilege writes here don't affect machined's other
// goroutines — the same guarantee the process runner relies on for dashboard/apid.
func beforeSandboxdExec(pa *syscall.ProcAttr, data any) error {
	cfg, ok := data.(*sandboxdLaunchContext)
	if !ok {
		return fmt.Errorf("failed to get sandboxd launch context")
	}

	pa.Files = cfg.files

	if pa.Sys == nil {
		pa.Sys = &syscall.SysProcAttr{}
	}

	pa.Sys.Cloneflags = syscall.CLONE_NEWPID | syscall.CLONE_NEWNS

	if selinux.IsEnabled() {
		if err := os.WriteFile("/proc/thread-self/attr/exec", []byte(constants.SelinuxLabelSandboxd), 0o777); err != nil {
			return fmt.Errorf("setting sandboxd SELinux exec label: %w", err)
		}
	}

	return nil
}
