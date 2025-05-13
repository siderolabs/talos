// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package process

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"slices"
	"syscall"
	"time"
	"unsafe"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/containerd/containerd/v2/pkg/sys"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-cmd/pkg/cmd/proc/reaper"
	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// processRunner is a runner.Runner that runs a process on the host.
type processRunner struct {
	args  *runner.Args
	opts  *runner.Options
	debug bool

	stop    chan struct{}
	stopped chan struct{}
}

// NewRunner creates runner.Runner that runs a process on the host.
func NewRunner(debug bool, args *runner.Args, setters ...runner.Option) runner.Runner {
	r := &processRunner{
		args:    args,
		opts:    runner.DefaultOptions(),
		debug:   debug,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}

	for _, setter := range setters {
		setter(r.opts)
	}

	return r
}

// Open implements the Runner interface.
func (p *processRunner) Open() error {
	return nil
}

// Run implements the Runner interface.
func (p *processRunner) Run(eventSink events.Recorder) error {
	defer close(p.stopped)

	return p.run(eventSink)
}

// Stop implements the Runner interface.
func (p *processRunner) Stop() error {
	close(p.stop)

	<-p.stopped

	p.stop = make(chan struct{})
	p.stopped = make(chan struct{})

	return nil
}

// Close implements the Runner interface.
func (p *processRunner) Close() error {
	return nil
}

type commandWrapper struct {
	launcher     *cap.Launcher
	ctty         optional.Optional[int]
	selinuxLabel string
	cgroupFile   *os.File
	stdin        *os.File
	stdout       *os.File
	stderr       *os.File
	afterStart   func()
}

func dropCaps(droppedCapabilities []string, launcher *cap.Launcher) error {
	dropCaps := xslices.Map(droppedCapabilities, func(c string) cap.Value {
		capability, capErr := cap.FromName(c)
		if capErr != nil {
			panic(fmt.Errorf("failed to parse capability: %s", capErr))
		}

		return capability
	})

	iab := cap.IABGetProc()
	if err := iab.SetVector(cap.Bound, true, dropCaps...); err != nil {
		return fmt.Errorf("failed to set capabilities: %w", err)
	}

	launcher.SetIAB(iab)

	return nil
}

// This callback is run in the thread before executing child process.
func beforeExecCallback(pa *syscall.ProcAttr, data any) error {
	wrapper, ok := data.(*commandWrapper)
	if !ok {
		return fmt.Errorf("failed to get command info")
	}

	ctty, cttySet := wrapper.ctty.Get()
	if cttySet {
		if pa.Sys == nil {
			pa.Sys = &syscall.SysProcAttr{}
		}

		pa.Sys.Ctty = ctty
		pa.Sys.Setsid = true
		pa.Sys.Setctty = true
	}

	pa.Files = []uintptr{
		wrapper.stdin.Fd(),
		wrapper.stdout.Fd(),
		wrapper.stderr.Fd(),
	}

	// It is only set in case we should use CgroupFD
	if wrapper.cgroupFile != nil {
		if pa.Sys == nil {
			pa.Sys = &syscall.SysProcAttr{}
		}

		pa.Sys.UseCgroupFD = true
		pa.Sys.CgroupFD = int(wrapper.cgroupFile.Fd())
	}

	// Use /proc/thread-self (Linux 3.17+) to avoid races between current
	// process threads leading to loss of the domain transition
	if selinux.IsEnabled() {
		if wrapper.selinuxLabel != "" {
			err := os.WriteFile("/proc/thread-self/attr/exec", []byte(wrapper.selinuxLabel), 0o777)
			if err != nil {
				log.Fatalf("%s", err)
			}
		} else {
			err := os.WriteFile("/proc/thread-self/attr/exec", []byte(constants.SelinuxLabelUnconfinedService), 0o777)
			if err != nil {
				log.Fatalf("%s", err)
			}
		}
	}

	return nil
}

//nolint:gocyclo
func (p *processRunner) build() (commandWrapper, error) {
	wrapper := commandWrapper{}

	env := slices.Concat([]string{"PATH=" + constants.PATH}, p.opts.Env, os.Environ())
	launcher := cap.NewLauncher(p.args.ProcessArgs[0], p.args.ProcessArgs, env)

	if p.opts.UID > 0 {
		launcher.SetUID(int(p.opts.UID))
	}

	// reduce capabilities and assign them to launcher
	if err := dropCaps(p.opts.DroppedCapabilities, launcher); err != nil {
		return commandWrapper{}, err
	}

	launcher.Callback(beforeExecCallback)

	// Setup logging.
	logSink, err := p.opts.LoggingManager.ServiceLog(p.args.ID).Writer()
	if err != nil {
		return commandWrapper{}, fmt.Errorf("service log handler: %w", err)
	}

	var logWriter io.Writer
	if p.debug {
		logWriter = io.MultiWriter(logSink, log.Writer())
	} else {
		logWriter = logSink
	}

	// As MultiWriter is not a file, we need to create a pipe
	// Pipe writer is passed to the child process while we read from the read side
	pr, pw, err := os.Pipe()
	if err != nil {
		return commandWrapper{}, err
	}

	go func() {
		defer pr.Close()      //nolint:errcheck
		defer logSink.Close() //nolint:errcheck

		io.Copy(logWriter, pr) //nolint:errcheck
	}()

	// close the writer if we exit early due to an error
	closeWriter := true

	afterStartClosers := []io.Closer{pw}

	closeLogging := func() {
		for _, closer := range afterStartClosers {
			closer.Close() //nolint:errcheck
		}
	}

	defer func() {
		if closeWriter {
			closeLogging()
		}
	}()

	if p.opts.StdinFile != "" {
		stdin, err := os.Open(p.opts.StdinFile)
		if err != nil {
			return commandWrapper{}, err
		}

		wrapper.stdin = stdin

		afterStartClosers = append(afterStartClosers, stdin)
	}

	if p.opts.StdoutFile != "" {
		stdout, err := os.OpenFile(p.opts.StdoutFile, os.O_WRONLY, 0)
		if err != nil {
			return commandWrapper{}, err
		}

		wrapper.stdout = stdout

		afterStartClosers = append(afterStartClosers, stdout)
	} else {
		// Do not close the fd in this case, it'll be done by closeLogger
		wrapper.stdout = pw
	}

	if p.opts.StderrFile != "" {
		stderr, err := os.OpenFile(p.opts.StderrFile, os.O_WRONLY, 0)
		if err != nil {
			return commandWrapper{}, err
		}

		wrapper.stderr = stderr

		afterStartClosers = append(afterStartClosers, stderr)
	} else {
		// Do not close the fd in this case, it'll be done by closeLogger
		wrapper.stderr = pw
	}

	closeWriter = false

	wrapper.launcher = launcher
	wrapper.afterStart = closeLogging
	wrapper.ctty = p.opts.Ctty
	wrapper.selinuxLabel = p.opts.SelinuxLabel

	cgroupFdSupported := false

	platform, err := platform.CurrentPlatform()
	if err == nil {
		cgroupFdSupported = platform.Mode() != runtime.ModeContainer
	}

	// cgroupfd is more reliable, use it when possible
	if cgroups.Mode() == cgroups.Unified && cgroupFdSupported && p.opts.UID == 0 {
		cg, err := os.Open(path.Join(constants.CgroupMountPath, cgroup.Path(p.opts.CgroupPath)))
		if err == nil {
			wrapper.cgroupFile = cg

			afterStartClosers = append(afterStartClosers, cg)
		}
	}

	return wrapper, nil
}

// Apply cgroup and OOM score after the process is launched.
//
//nolint:gocyclo,cyclop
func applyProperties(p *processRunner, pid int) error {
	if p.opts.CgroupPath != "" {
		path := cgroup.Path(p.opts.CgroupPath)

		if cgroups.Mode() == cgroups.Unified {
			cgv2, err := cgroup2.Load(path)
			if err != nil {
				return fmt.Errorf("failed to load cgroup %s: %w", path, err)
			}

			// No such process error can happen in case the process is terminated before this code runs
			if err := cgv2.AddProc(uint64(pid)); err != nil {
				pathError, ok := err.(*fs.PathError)
				if !ok || pathError.Err != syscall.ESRCH {
					return fmt.Errorf("failed to move process %s to cgroup: %w", p, err)
				}
			}
		} else {
			cgv1, err := cgroup1.Load(cgroup1.StaticPath(path))
			if err != nil {
				return fmt.Errorf("failed to load cgroup %s: %w", path, err)
			}

			if err := cgv1.Add(cgroup1.Process{
				Pid: pid,
			}); err != nil {
				pathError, ok := err.(*fs.PathError)
				if !ok || pathError.Err != syscall.ESRCH {
					return fmt.Errorf("failed to move process %s to cgroup: %w", p, err)
				}
			}
		}
	}

	if p.opts.OOMScoreAdj != 0 {
		if err := sys.AdjustOOMScore(pid, p.opts.OOMScoreAdj); err != nil {
			pathError, ok := err.(*fs.PathError)
			if !ok || pathError.Err != syscall.ENOENT {
				return fmt.Errorf("failed to change OOMScoreAdj of process %s to %d: %w", p, p.opts.OOMScoreAdj, err)
			}
		}
	}

	if p.opts.Priority != 0 {
		if err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, p.opts.Priority); err != nil {
			return fmt.Errorf("failed to set priority of process %s to %d: %w", p, p.opts.Priority, err)
		}
	}

	if ioPriority, ioPrioritySet := p.opts.IOPriority.Get(); ioPrioritySet {
		err := setIOPriority(p, pid, ioPriority)
		if err != nil {
			return err
		}
	}

	if schedulingPolicy, schedulingPolicySet := p.opts.SchedulingPolicy.Get(); schedulingPolicySet {
		err := setSchedulingPolicy(p, pid, schedulingPolicy)
		if err != nil {
			return err
		}
	}

	return nil
}

func setIOPriority(p *processRunner, pid int, ioPriority runner.IOPriorityParam) error {
	if ioPriority.Class > runner.IoprioClassIdle {
		return fmt.Errorf("failed to set IO priority of process %s: class %d is not valid", p, ioPriority.Class)
	}

	if ioPriority.Priority > 7 {
		return fmt.Errorf("failed to set IO priority of process %s: priority %d is not valid", p, ioPriority.Priority)
	}

	classPos := 13 // IOPRIO_CLASS_SHIFT
	priorityValue := ioPriority.Class<<classPos | ioPriority.Priority
	sysctlWho := uintptr(1) // IOPRIO_WHO_PROCESS, we don't operate on threads or groups

	ret, _, syscallError := syscall.Syscall(syscall.SYS_IOPRIO_SET, sysctlWho, uintptr(pid), uintptr(priorityValue))
	if int(ret) == -1 {
		return fmt.Errorf("failed to set IO priority of process %s to %d: syscall failed with %s", p, priorityValue, syscallError.Error())
	}

	return nil
}

func setSchedulingPolicy(p *processRunner, pid int, schedulingPolicy uint) error {
	if schedulingPolicy > runner.SchedulingPolicyDeadline {
		return fmt.Errorf("failed to set scheduling policy of process %s: policy %d is not valid", p, schedulingPolicy)
	}

	options := struct{ Priority int32 }{
		Priority: int32(0),
	}

	if _, _, syscallError := syscall.Syscall(
		syscall.SYS_SCHED_SETSCHEDULER,
		uintptr(pid),
		uintptr(schedulingPolicy),
		uintptr(unsafe.Pointer(&options)),
	); syscallError != 0 {
		return fmt.Errorf("failed to set scheduling policy of process %s to %d: syscall failed with %s", p, schedulingPolicy, syscallError.Error())
	}

	return nil
}

func (p *processRunner) run(eventSink events.Recorder) error {
	cmdWrapper, err := p.build()
	if err != nil {
		return fmt.Errorf("error building command: %w", err)
	}

	defer cmdWrapper.afterStart()

	notifyCh := make(chan reaper.ProcessInfo, 8)

	usingReaper := reaper.Notify(notifyCh)
	if usingReaper {
		defer reaper.Stop(notifyCh)
	}

	pid, err := cmdWrapper.launcher.Launch(&cmdWrapper)
	if err != nil {
		return fmt.Errorf("error starting process: %w", err)
	}

	if err := applyProperties(p, pid); err != nil {
		return err
	}

	cmdWrapper.afterStart()

	eventSink(events.StateRunning, "Process %s started with PID %d", p, pid)

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("could not find process: %w", err)
	}

	waitCh := make(chan error)

	go func() {
		waitCh <- reaper.ProcessWaitWrapper(usingReaper, notifyCh, process)
	}()

	select {
	case err = <-waitCh:
		// process exited
		return err
	case <-p.stop:
		// graceful stop the service
		eventSink(events.StateStopping, "Sending SIGTERM to %s", p)

		//nolint:errcheck
		_ = process.Signal(syscall.SIGTERM)
	}

	select {
	case <-waitCh:
		// stopped process exited
		return nil
	case <-time.After(p.opts.GracefulShutdownTimeout):
		// kill the process
		eventSink(events.StateStopping, "Sending SIGKILL to %s", p)

		//nolint:errcheck
		_ = process.Signal(syscall.SIGKILL)
	}

	// wait for process to terminate
	<-waitCh

	return nil
}

func (p *processRunner) String() string {
	return fmt.Sprintf("Process(%q)", p.args.ProcessArgs)
}
