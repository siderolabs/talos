// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package process

import (
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/containerd/containerd/v2/pkg/sys"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-cmd/pkg/cmd/proc/reaper"
	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
	krnl "github.com/siderolabs/talos/pkg/kernel"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
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
	launcher         *cap.Launcher
	ctty             optional.Optional[int]
	stdin            uintptr
	stdout           uintptr
	stderr           uintptr
	afterStart       func()
	afterTermination func() error
}

func dropCaps(droppedCapabilities []string, launcher *cap.Launcher) error {
	droppedCaps := strings.Join(droppedCapabilities, ",")

	prop, err := krnl.ReadParam(&kernel.Param{Key: "proc.sys.kernel.kexec_load_disabled"})
	if v := strings.TrimSpace(string(prop)); err == nil && v != "0" {
		log.Printf("kernel.kexec_load_disabled is %s, skipping dropping capabilities", v)
	} else if droppedCaps != "" {
		caps := strings.Split(droppedCaps, ",")
		dropCaps := xslices.Map(caps, func(c string) cap.Value {
			capability, capErr := cap.FromName(c)
			if capErr != nil {
				fmt.Printf("failed to parse capability: %s", capErr)
			}

			return capability
		})

		iab := cap.IABGetProc()
		if err = iab.SetVector(cap.Bound, true, dropCaps...); err != nil {
			return fmt.Errorf("failed to set capabilities: %s", err)
		}

		launcher.SetIAB(iab)
	}

	return nil
}

// This callback is run in the thread before executing child process.
func beforeExecCallback(pa *syscall.ProcAttr, data interface{}) error {
	wrapper, ok := data.(*commandWrapper)
	if !ok {
		return fmt.Errorf("failed to get command info")
	}

	ctty, cttySet := wrapper.ctty.Get()
	if cttySet {
		pa.Sys.Ctty = ctty
		pa.Sys.Setsid = true
		pa.Sys.Setctty = true
	}

	pa.Files = []uintptr{
		wrapper.stdin,
		wrapper.stdout,
		wrapper.stderr,
	}

	// TODO: use pa.Sys.CgroupFD here when we can be sure clone3 is available
	fmt.Println("Callback executed")

	return nil
}

//nolint:gocyclo
func (p *processRunner) build() (commandWrapper, error) {
	wrapper := commandWrapper{}

	env := slices.Concat([]string{"PATH=" + constants.PATH}, p.opts.Env, os.Environ())
	launcher := cap.NewLauncher(p.args.ProcessArgs[0], p.args.ProcessArgs[1:], env)

	if p.opts.UID > 0 {
		launcher.SetUID(int(p.opts.UID))
	}

	// reduce capabilities and assign them to launcher
	if err := dropCaps(p.opts.DroppedCapabilities, launcher); err != nil {
		return commandWrapper{}, err
	}

	launcher.Callback(beforeExecCallback)

	// Setup logging.
	w, err := p.opts.LoggingManager.ServiceLog(p.args.ID).Writer()
	if err != nil {
		return commandWrapper{}, fmt.Errorf("service log handler: %w", err)
	}

	var writer io.Writer
	if p.debug { // TODO: wrap it into LoggingManager
		writer = io.MultiWriter(w, os.Stdout)
	} else {
		writer = w
	}

	// As MultiWriter is not a file, we need to create a pipe
	// Pipe writer is passed to the child process while we read from the read side
	pr, pw, err := os.Pipe()
	if err != nil {
		return commandWrapper{}, err
	}

	go io.Copy(writer, pr) //nolint:errcheck

	// close the writer if we exit early due to an error
	closeWriter := true

	closeLogging := func() (e error) {
		err := w.Close()
		if err != nil {
			e = err
		}

		err = pr.Close()
		if err != nil {
			e = err
		}

		err = pw.Close()
		if err != nil {
			e = err
		}

		return e
	}

	defer func() {
		if closeWriter {
			closeLogging() //nolint:errcheck
		}
	}()

	var afterStartFuncs []func()

	if p.opts.StdinFile != "" {
		stdin, err := os.Open(p.opts.StdinFile)
		if err != nil {
			return commandWrapper{}, err
		}

		wrapper.stdin = stdin.Fd()

		afterStartFuncs = append(afterStartFuncs, func() {
			stdin.Close() //nolint:errcheck
		})
	}

	if p.opts.StdoutFile != "" {
		stdout, err := os.OpenFile(p.opts.StdoutFile, os.O_WRONLY, 0)
		if err != nil {
			return commandWrapper{}, err
		}

		wrapper.stdout = stdout.Fd()

		afterStartFuncs = append(afterStartFuncs, func() {
			stdout.Close() //nolint:errcheck
		})
	} else {
		wrapper.stdout = pw.Fd()
	}

	if p.opts.StderrFile != "" {
		stderr, err := os.OpenFile(p.opts.StderrFile, os.O_WRONLY, 0)
		if err != nil {
			return commandWrapper{}, err
		}

		wrapper.stderr = stderr.Fd()

		afterStartFuncs = append(afterStartFuncs, func() {
			stderr.Close() //nolint:errcheck
		})
	} else {
		wrapper.stderr = pw.Fd()
	}

	closeWriter = false

	wrapper.launcher = launcher
	wrapper.afterStart = func() {
		for _, f := range afterStartFuncs {
			f()
		}
	}
	wrapper.afterTermination = closeLogging
	wrapper.ctty = p.opts.Ctty

	return wrapper, nil
}

// Apply cgroup and OOM score after the process is launched.
func applyProperties(p *processRunner, pid int) error {
	path := cgroup.Path(p.opts.CgroupPath)

	if cgroups.Mode() == cgroups.Unified {
		cgv2, err := cgroup2.Load(path)
		if err != nil {
			return fmt.Errorf("failed to load cgroup %s: %s", path, err)
		}

		if err := cgv2.AddProc(uint64(pid)); err != nil {
			return fmt.Errorf("failed to move process %s to cgroup: %s", p, err)
		}
	} else {
		cgv1, err := cgroup1.Load(cgroup1.StaticPath(path))
		if err != nil {
			return fmt.Errorf("failed to load cgroup %s: %s", path, err)
		}

		if err := cgv1.Add(cgroup1.Process{
			Pid: pid,
		}); err != nil {
			return fmt.Errorf("failed to move process %s to cgroup: %s", p, err)
		}
	}

	if err := sys.AdjustOOMScore(pid, p.opts.OOMScoreAdj); err != nil {
		return fmt.Errorf("failed to change OOMScoreAdj of process %s to %d", p, p.opts.OOMScoreAdj)
	}

	return nil
}

func (p *processRunner) run(eventSink events.Recorder) error {
	cmdWrapper, err := p.build()
	if err != nil {
		return fmt.Errorf("error building command: %w", err)
	}

	defer cmdWrapper.afterTermination() //nolint:errcheck

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
		_, err := process.Wait()
		waitCh <- err
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

	return cmdWrapper.afterTermination()
}

func (p *processRunner) String() string {
	return fmt.Sprintf("Process(%q)", p.args.ProcessArgs)
}
