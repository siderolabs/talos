// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// To disable the `stutter` check
// nolint: golint
package local

import (
	"context"
	"fmt"
	"time"

	"github.com/talos-systems/talos/internal/test-framework/pkg/checker"
	"github.com/talos-systems/talos/internal/test-framework/pkg/runner"
	"github.com/talos-systems/talos/pkg/retry"
)

// LocalRunner implements the Runner interface.
type LocalRunner struct {
}

// New creates a new local runner with the specified options.
func New(setters ...Option) (runner.Runner, error) {
	return &LocalRunner{}, nil
}

// Run invokes a one off command on the host.
func (lr *LocalRunner) Run(ctx context.Context, check checker.Check) (err error) {
	check.Command.Stdout = &check.Stdout
	check.Command.Stderr = &check.Stderr

	return check.Command.Run()
}

// Check invokes a command with a specified check function to validate
// the command was executed correctly.
func (lr *LocalRunner) Check(ctx context.Context, check checker.Check) (err error) {
	check.Command.Stdout = &check.Stdout
	check.Command.Stderr = &check.Stderr

	err = retry.Exponential(check.Wait, retry.WithUnits(250*time.Millisecond), retry.WithJitter(50*time.Millisecond)).Retry(func() error {
		check.Stdout.Reset()
		check.Stderr.Reset()

		if err = check.Command.Start(); err != nil {
			return retry.UnexpectedError(err)
		}

		// Retry if command doesnt exit 0
		if err = check.Command.Wait(); err != nil {
			return retry.ExpectedError(err)
		}

		// Verify output is what we expect
		if !check.Check(check.Stdout.String()) {
			return retry.ExpectedError(fmt.Errorf("check was not successful\nstderr:%s\nstdout:%s", check.Stderr.String(), check.Stdout.String()))
		}

		return nil
	})

	return err
}

// Cleanup is used to remove the container and any associated resources for the runner.
// Since a local runner just invokes a command on the host, no cleanup will be performed.
func (lr *LocalRunner) Cleanup(ctx context.Context) (err error) { return err }
