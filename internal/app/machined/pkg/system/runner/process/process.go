// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package process

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/siderolabs/go-cmd/pkg/cmd/proc/reaper"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
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
	cmd              *exec.Cmd
	afterStart       func()
	afterTermination func() error
}

//nolint:gocyclo
func (p *processRunner) build() (commandWrapper, error) {
	args := []string{
		fmt.Sprintf("-name=%s", p.args.ID),
		fmt.Sprintf("-dropped-caps=%s", strings.Join(p.opts.DroppedCapabilities, ",")),
		fmt.Sprintf("-cgroup-path=%s", cgroup.Path(p.opts.CgroupPath)),
		fmt.Sprintf("-oom-score=%d", p.opts.OOMScoreAdj),
		fmt.Sprintf("-uid=%d", p.opts.UID),
	}

	args = append(args, p.args.ProcessArgs...)

	cmd := exec.Command("/sbin/wrapperd", args...)

	// Set the environment for the service.
	cmd.Env = append([]string{fmt.Sprintf("PATH=%s", constants.PATH)}, p.opts.Env...)

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

	// close the writer if we exit early due to an error
	closeWriter := true

	defer func() {
		if closeWriter {
			w.Close() //nolint:errcheck
		}
	}()

	var afterStartFuncs []func()

	if p.opts.StdinFile != "" {
		stdin, err := os.Open(p.opts.StdinFile)
		if err != nil {
			return commandWrapper{}, err
		}

		cmd.Stdin = stdin

		afterStartFuncs = append(afterStartFuncs, func() {
			stdin.Close() //nolint:errcheck
		})
	}

	if p.opts.StdoutFile != "" {
		stdout, err := os.OpenFile(p.opts.StdoutFile, os.O_WRONLY, 0)
		if err != nil {
			return commandWrapper{}, err
		}

		cmd.Stdout = stdout

		afterStartFuncs = append(afterStartFuncs, func() {
			stdout.Close() //nolint:errcheck
		})
	} else {
		cmd.Stdout = writer
	}

	if p.opts.StderrFile != "" {
		stderr, err := os.OpenFile(p.opts.StderrFile, os.O_WRONLY, 0)
		if err != nil {
			return commandWrapper{}, err
		}

		cmd.Stderr = stderr

		afterStartFuncs = append(afterStartFuncs, func() {
			stderr.Close() //nolint:errcheck
		})
	} else {
		cmd.Stderr = writer
	}

	ctty, cttySet := p.opts.Ctty.Get()
	if cttySet {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
			Ctty:    ctty,
		}
	}

	closeWriter = false

	return commandWrapper{
		cmd: cmd,
		afterStart: func() {
			for _, f := range afterStartFuncs {
				f()
			}
		},
		afterTermination: func() error {
			return w.Close()
		},
	}, nil
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

	err = cmdWrapper.cmd.Start()

	cmdWrapper.afterStart()

	if err != nil {
		return fmt.Errorf("error starting process: %w", err)
	}

	eventSink(events.StateRunning, "Process %s started with PID %d", p, cmdWrapper.cmd.Process.Pid)

	waitCh := make(chan error)

	go func() {
		waitCh <- reaper.WaitWrapper(usingReaper, notifyCh, cmdWrapper.cmd)
	}()

	select {
	case err = <-waitCh:
		// process exited
		return err
	case <-p.stop:
		// graceful stop the service
		eventSink(events.StateStopping, "Sending SIGTERM to %s", p)

		//nolint:errcheck
		_ = cmdWrapper.cmd.Process.Signal(syscall.SIGTERM)
	}

	select {
	case <-waitCh:
		// stopped process exited
		return nil
	case <-time.After(p.opts.GracefulShutdownTimeout):
		// kill the process
		eventSink(events.StateStopping, "Sending SIGKILL to %s", p)

		//nolint:errcheck
		_ = cmdWrapper.cmd.Process.Signal(syscall.SIGKILL)
	}

	// wait for process to terminate
	<-waitCh

	return cmdWrapper.afterTermination()
}

func (p *processRunner) String() string {
	return fmt.Sprintf("Process(%q)", p.args.ProcessArgs)
}
