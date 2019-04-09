/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package process

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	processlogger "github.com/talos-systems/talos/internal/app/init/pkg/system/runner/process/log"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// processRunner is a runner.Runner that runs a process on the host.
type processRunner struct {
	data *userdata.UserData
	args *runner.Args
	opts *runner.Options

	stop    chan struct{}
	stopped chan struct{}
}

// errStopped is used internally to signal that task was stopped
var errStopped = errors.New("stopped")

// NewRunner creates runner.Runner that runs a process on the host
func NewRunner(data *userdata.UserData, args *runner.Args, setters ...runner.Option) runner.Runner {
	r := &processRunner{
		data:    data,
		args:    args,
		opts:    runner.DefaultOptions(),
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}

	for _, setter := range setters {
		setter(r.opts)
	}

	return r
}

// Run implements the Runner interface.
func (p *processRunner) Run() error {
	defer close(p.stopped)

	switch p.opts.Type {
	case runner.Forever:
		p.waitAndRestart()
	case runner.Once:
		p.waitForSuccess()
	default:
		panic("unexpected runner type")
	}

	return nil
}

// Stop implements the Runner interface
func (p *processRunner) Stop() error {
	close(p.stop)

	<-p.stopped

	return nil
}

func (p *processRunner) build() (cmd *exec.Cmd, err error) {
	cmd = exec.Command(p.args.ProcessArgs[0], p.args.ProcessArgs[1:]...)

	// Set the environment for the service.
	cmd.Env = append([]string{fmt.Sprintf("PATH=%s", constants.PATH)}, p.opts.Env...)

	// Setup logging.
	w, err := processlogger.New(p.args.ID, p.opts.LogPath)
	if err != nil {
		err = fmt.Errorf("service log handler: %v", err)
		return
	}

	var writer io.Writer
	if p.data.Debug {
		writer = io.MultiWriter(w, os.Stdout)
	} else {
		writer = w
	}
	cmd.Stdout = writer
	cmd.Stderr = writer

	return cmd, nil
}

func (p *processRunner) run() error {
	cmd, err := p.build()
	if err != nil {
		return errors.Wrap(err, "error building command")
	}

	if err = cmd.Start(); err != nil {
		return errors.Wrap(err, "error starting process")
	}

	waitCh := make(chan error)

	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err = <-waitCh:
		// process exited
		return err
	case <-p.stop:
		// graceful stop the service

		// nolint: errcheck
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}

	select {
	case <-waitCh:
		// stopped process exited
		return errStopped
	case <-time.After(p.opts.GracefulShutdownTimeout):
		// kill the process

		// nolint: errcheck
		_ = cmd.Process.Kill()
	}

	// wait for process to terminate
	<-waitCh
	return errStopped
}

func (p *processRunner) waitAndRestart() {
	for {
		err := p.run()
		if err == errStopped {
			return
		}
		if err != nil {
			log.Printf("error running %v, going to restart forever: %s", p.args.ProcessArgs, err)
		}

		select {
		case <-p.stop:
			return
		case <-time.After(p.opts.RestartInterval):
		}
	}
}

func (p *processRunner) waitForSuccess() {
	for {
		err := p.run()
		if err == errStopped || err == nil {
			break
		}

		log.Printf("error running %v, going to restart until it succeeds: %s", p.args.ProcessArgs, err)

		select {
		case <-p.stop:
			return
		case <-time.After(p.opts.RestartInterval):
		}
	}
}
