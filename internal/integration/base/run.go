// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package base

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/stretchr/testify/suite"
)

// RunOption configures options for Run
type RunOption func(*runOptions)

type runOptions struct {
	shouldFail     bool
	stderrNotEmpty bool
	stdoutRegexps  []*regexp.Regexp
}

// ShouldFail tells Run command should fail.
//
// ShouldFail also sets StdErrNotEmpty.
func ShouldFail() RunOption {
	return func(opts *runOptions) {
		opts.shouldFail = true
	}
}

// ShouldSucceed tells Run command should succeed (that is default).
func ShouldSucceed() RunOption {
	return func(opts *runOptions) {
		opts.shouldFail = true
	}
}

// StderrNotEmpty tells run that stderr of the command should not be empty.
func StdErrNotEmpty() RunOption {
	return func(opts *runOptions) {
		opts.stderrNotEmpty = true
	}
}

// StdoutShouldMatch appends to the set of regexps stdout contents should match.
func StdoutShouldMatch(r *regexp.Regexp) RunOption {
	return func(opts *runOptions) {
		opts.stdoutRegexps = append(opts.stdoutRegexps, r)
	}
}

// Run executes command and asserts on its exit status/output
func Run(suite *suite.Suite, cmd *exec.Cmd, options ...RunOption) {
	var opts runOptions

	for _, o := range options {
		o(&opts)
	}

	var stdout, stderr bytes.Buffer

	cmd.Stdin = nil
	cmd.Stdout = &stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	cmd.Env = []string{}

	// filter environment variables
	for _, keyvalue := range os.Environ() {
		index := strings.Index(keyvalue, "=")
		if index < 0 {
			continue
		}
		switch strings.ToUpper(keyvalue[:index]) {
		case "PATH":
			fallthrough
		case "HOME":
			fallthrough
		case "USERNAME":
			cmd.Env = append(cmd.Env, keyvalue)
		}
	}

	suite.Require().NoError(cmd.Start())

	err := cmd.Wait()

	if err == nil {
		if opts.shouldFail {
			suite.Assert().NotNil(err, "command should have failed but it exited with zero exit code")
		}
	} else {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			suite.Require().Nil(err, "command failed to be run")
		}

		if !opts.shouldFail {
			suite.Assert().Nil(exitErr, "command failed with exit code: %d", exitErr.ExitCode())
		}
	}

	if opts.stderrNotEmpty {
		suite.Assert().NotEmpty(stderr.String(), "stderr should be not empty")
	} else {
		suite.Assert().Empty(stderr.String(), "stderr should be empty")
	}

	for _, rx := range opts.stdoutRegexps {
		suite.Assert().Regexp(rx, stdout.String())
	}
}
