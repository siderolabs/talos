// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/talos-systems/go-cmd/pkg/cmd/proc/reaper"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/pkg/machinery/constants"
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
func (p *processRunner) Open(ctx context.Context) error {
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

func (p *processRunner) build() (cmd *exec.Cmd, logCloser io.Closer, err error) {
	cmd = exec.Command(p.args.ProcessArgs[0], p.args.ProcessArgs[1:]...)

	// Set the environment for the service.
	cmd.Env = append([]string{fmt.Sprintf("PATH=%s", constants.PATH)}, p.opts.Env...)

	// Setup logging.
	w, err := p.opts.LoggingManager.ServiceLog(p.args.ID).Writer()
	if err != nil {
		err = fmt.Errorf("service log handler: %w", err)

		return
	}

	var writer io.Writer
	if p.debug { // TODO: wrap it into LoggingManager
		writer = io.MultiWriter(w, os.Stdout)
	} else {
		writer = w
	}

	cmd.Stdout = writer
	cmd.Stderr = writer

	return cmd, w, nil
}

func (p *processRunner) run(eventSink events.Recorder) error {
	cmd, logCloser, err := p.build()
	if err != nil {
		return fmt.Errorf("error building command: %w", err)
	}

	defer logCloser.Close() //nolint:errcheck

	notifyCh := make(chan reaper.ProcessInfo, 8)

	usingReaper := reaper.Notify(notifyCh)
	if usingReaper {
		defer reaper.Stop(notifyCh)
	}

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("error starting process: %w", err)
	}

	eventSink(events.StateRunning, "Process %s started with PID %d", p, cmd.Process.Pid)

	waitCh := make(chan error)

	go func() {
		waitCh <- reaper.WaitWrapper(usingReaper, notifyCh, cmd)
	}()

	select {
	case err = <-waitCh:
		// process exited
		return err
	case <-p.stop:
		// graceful stop the service
		eventSink(events.StateStopping, "Sending SIGTERM to %s", p)

		//nolint:errcheck
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}

	select {
	case <-waitCh:
		// stopped process exited
		return nil
	case <-time.After(p.opts.GracefulShutdownTimeout):
		// kill the process
		eventSink(events.StateStopping, "Sending SIGKILL to %s", p)

		//nolint:errcheck
		_ = cmd.Process.Signal(syscall.SIGKILL)
	}

	// wait for process to terminate
	<-waitCh

	return logCloser.Close()
}

func (p *processRunner) String() string {
	return fmt.Sprintf("Process(%q)", p.args.ProcessArgs)
}
