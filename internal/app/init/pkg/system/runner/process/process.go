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
	"time"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	processlogger "github.com/talos-systems/talos/internal/app/init/pkg/system/runner/process/log"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Process is a runner.Runner that runs a process on the host.
type Process struct{}

// Run implements the Runner interface.
func (p *Process) Run(data *userdata.UserData, args *runner.Args, setters ...runner.Option) error {
	opts := runner.DefaultOptions()
	for _, setter := range setters {
		setter(opts)
	}

	switch opts.Type {
	case runner.Forever:
		if err := p.waitAndRestart(data, args, opts); err != nil {
			return err
		}
	case runner.Once:
		if err := p.waitForSuccess(data, args, opts); err != nil {
			return err
		}
	}

	return nil
}

func (p *Process) build(data *userdata.UserData, args *runner.Args, opts *runner.Options) (cmd *exec.Cmd, err error) {
	cmd = exec.Command(args.ProcessArgs[0], args.ProcessArgs[1:]...)

	// Set the environment for the service.
	cmd.Env = append([]string{fmt.Sprintf("PATH=%s", constants.PATH)}, opts.Env...)

	// Setup logging.
	w, err := processlogger.New(args.ID)
	if err != nil {
		err = fmt.Errorf("service log handler: %v", err)
		return
	}

	var writer io.Writer
	if data.Debug {
		writer = io.MultiWriter(w, os.Stdout)
	} else {
		writer = w
	}
	cmd.Stdout = writer
	cmd.Stderr = writer

	return cmd, nil
}

func (p *Process) run(data *userdata.UserData, args *runner.Args, opts *runner.Options) error {
	cmd, err := p.build(data, args, opts)
	if err != nil {
		return errors.Wrap(err, "error building command")
	}

	if err = cmd.Start(); err != nil {
		return errors.Wrap(err, "error starting process")
	}

	return cmd.Wait()
}

func (p *Process) waitAndRestart(data *userdata.UserData, args *runner.Args, opts *runner.Options) error {
	for {
		err := p.run(data, args, opts)
		if err != nil {
			log.Printf("error running %v, going to restart forever: %s", args.ProcessArgs, err)
		}

		time.Sleep(5 * time.Second)
	}
}

func (p *Process) waitForSuccess(data *userdata.UserData, args *runner.Args, opts *runner.Options) error {
	for {
		err := p.run(data, args, opts)
		if err == nil {
			break
		}

		log.Printf("error running %v, going to restart until it succeeds: %s", args.ProcessArgs, err)
		time.Sleep(5 * time.Second)
	}

	return nil
}
