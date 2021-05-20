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

// RunOption configures options for Run.
type RunOption func(*runOptions)

// MatchFunc runs against output (stdout or stderr).
type MatchFunc func(output string) error

type runOptions struct {
	shouldFail            bool
	stdoutEmpty           bool
	stderrNotEmpty        bool
	stdoutRegexps         []*regexp.Regexp
	stderrRegexps         []*regexp.Regexp
	stdoutNegativeRegexps []*regexp.Regexp
	stderrNegativeRegexps []*regexp.Regexp
	stdoutMatchers        []MatchFunc
	stderrMatchers        []MatchFunc
}

// ShouldFail tells Run command should fail.
//
// ShouldFail also sets StdErrNotEmpty.
func ShouldFail() RunOption {
	return func(opts *runOptions) {
		opts.shouldFail = true
	}
}

// StderrNotEmpty tells run that stderr of the command should not be empty.
func StderrNotEmpty() RunOption {
	return func(opts *runOptions) {
		opts.stderrNotEmpty = true
	}
}

// StdoutEmpty tells run that stdout of the command should be empty.
func StdoutEmpty() RunOption {
	return func(opts *runOptions) {
		opts.stdoutEmpty = true
	}
}

// StdoutShouldMatch appends to the set of regexps stdout contents should match.
func StdoutShouldMatch(r *regexp.Regexp) RunOption {
	return func(opts *runOptions) {
		opts.stdoutRegexps = append(opts.stdoutRegexps, r)
	}
}

// StdoutShouldNotMatch appends to the set of regexps stdout contents should not match.
func StdoutShouldNotMatch(r *regexp.Regexp) RunOption {
	return func(opts *runOptions) {
		opts.stdoutNegativeRegexps = append(opts.stdoutNegativeRegexps, r)
	}
}

// StderrShouldMatch appends to the set of regexps sterr contents should match.
func StderrShouldMatch(r *regexp.Regexp) RunOption {
	return func(opts *runOptions) {
		opts.stderrRegexps = append(opts.stderrRegexps, r)
	}
}

// StderrShouldNotMatch appends to the set of regexps sterr contents should not match.
func StderrShouldNotMatch(r *regexp.Regexp) RunOption {
	return func(opts *runOptions) {
		opts.stderrNegativeRegexps = append(opts.stderrNegativeRegexps, r)
	}
}

// StdoutMatchFunc appends to the list of MatchFuncs to run against stdout.
func StdoutMatchFunc(f MatchFunc) RunOption {
	return func(opts *runOptions) {
		opts.stdoutMatchers = append(opts.stdoutMatchers, f)
	}
}

// StderrMatchFunc appends to the list of MatchFuncs to run against stderr.
func StderrMatchFunc(f MatchFunc) RunOption {
	return func(opts *runOptions) {
		opts.stderrMatchers = append(opts.stderrMatchers, f)
	}
}

// runAndWait launches the command and waits for completion.
//
// runAndWait doesn't do any assertions on result.
func runAndWait(suite *suite.Suite, cmd *exec.Cmd) (stdoutBuf, stderrBuf *bytes.Buffer, err error) {
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

	err = cmd.Wait()

	return &stdout, &stderr, err
}

// Run executes command, asserts on its exit status/output, and returns stdout.
//
//nolint:gocyclo,nakedret
func Run(suite *suite.Suite, cmd *exec.Cmd, options ...RunOption) (stdout string) {
	var opts runOptions

	for _, o := range options {
		o(&opts)
	}

	stdoutBuf, stderrBuf, err := runAndWait(suite, cmd)

	if stdoutBuf != nil {
		stdout = stdoutBuf.String()
	}

	var stderr string
	if stderrBuf != nil {
		stderr = stderrBuf.String()
	}

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

	if opts.stdoutEmpty {
		suite.Assert().Empty(stdout, "stdout should be empty")
	} else {
		suite.Assert().NotEmpty(stdout, "stdout should be not empty")
	}

	if opts.stderrNotEmpty {
		suite.Assert().NotEmpty(stderr, "stderr should be not empty")
	} else {
		suite.Assert().Empty(stderr, "stderr should be empty")
	}

	for _, rx := range opts.stdoutRegexps {
		suite.Assert().Regexp(rx, stdout)
	}

	for _, rx := range opts.stderrRegexps {
		suite.Assert().Regexp(rx, stderr)
	}

	for _, rx := range opts.stdoutNegativeRegexps {
		suite.Assert().NotRegexp(rx, stdout)
	}

	for _, rx := range opts.stderrNegativeRegexps {
		suite.Assert().NotRegexp(rx, stderr)
	}

	for _, f := range opts.stdoutMatchers {
		suite.Assert().NoError(f(stdout), "stdout match: %q", stdout)
	}

	for _, f := range opts.stderrMatchers {
		suite.Assert().NoError(f(stderr), "stderr match: %q", stderr)
	}

	return
}
