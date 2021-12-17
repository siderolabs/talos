// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli
// +build integration_cli

package base

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"
)

// RunOption configures options for Run.
type RunOption func(*runOptions)

// MatchFunc runs against output (stdout or stderr).
type MatchFunc func(output string) error

type runOptions struct {
	retryer               retry.Retryer
	shouldFail            bool
	stdoutEmpty           bool
	stderrNotEmpty        bool
	stdoutRegexps         []*regexp.Regexp
	stdoutNegativeRegexps []*regexp.Regexp
	stderrRegexps         []*regexp.Regexp
	stderrNegativeRegexps []*regexp.Regexp
	stdoutMatchers        []MatchFunc
	stderrMatchers        []MatchFunc
}

// WithRetry retries failing command runs.
func WithRetry(retryer retry.Retryer) RunOption {
	return func(opts *runOptions) {
		opts.retryer = retryer
	}
}

// ShouldFail tells run command should fail (with non-empty stderr).
//
// ShouldFail also sets StdErrNotEmpty.
func ShouldFail() RunOption {
	return func(opts *runOptions) {
		opts.shouldFail = true
		opts.stderrNotEmpty = true
	}
}

// StdoutEmpty tells run that stdout of the command should be empty.
func StdoutEmpty() RunOption {
	return func(opts *runOptions) {
		opts.stdoutEmpty = true
	}
}

// StderrNotEmpty tells run that stderr of the command should not be empty.
func StderrNotEmpty() RunOption {
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

// StdoutShouldNotMatch appends to the set of regexps stdout contents should not match.
func StdoutShouldNotMatch(r *regexp.Regexp) RunOption {
	return func(opts *runOptions) {
		opts.stdoutNegativeRegexps = append(opts.stdoutNegativeRegexps, r)
	}
}

// StderrShouldMatch appends to the set of regexps stderr contents should match.
//
// StderrShouldMatch also sets StdErrNotEmpty.
func StderrShouldMatch(r *regexp.Regexp) RunOption {
	return func(opts *runOptions) {
		opts.stderrRegexps = append(opts.stderrRegexps, r)
		opts.stderrNotEmpty = true
	}
}

// StderrShouldNotMatch appends to the set of regexps stderr contents should not match.
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
	cmd.Stderr = &stderr
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

	suite.T().Logf("Running %q", strings.Join(cmd.Args, " "))

	suite.Require().NoError(cmd.Start())

	err = cmd.Wait()

	return &stdout, &stderr, err
}

// retryRunAndWait retries runAndWait if the command fails to run.
func retryRunAndWait(suite *suite.Suite, cmd *exec.Cmd, retryer retry.Retryer) (stdoutBuf, stderrBuf *bytes.Buffer, err error) {
	err = retryer.Retry(func() error {
		stdoutBuf, stderrBuf, err = runAndWait(suite, cmd)

		if _, ok := err.(*exec.ExitError); ok {
			return retry.ExpectedError(err)
		}

		return err
	})

	return
}

// run executes command, asserts on its exit status/output, and returns stdout.
//
//nolint:gocyclo,nakedret
func run(suite *suite.Suite, cmd *exec.Cmd, options ...RunOption) (stdout string) {
	var opts runOptions

	for _, o := range options {
		o(&opts)
	}

	var (
		stdoutBuf, stderrBuf *bytes.Buffer
		err                  error
	)

	if opts.retryer != nil {
		stdoutBuf, stderrBuf, err = retryRunAndWait(suite, cmd, opts.retryer)
	} else {
		stdoutBuf, stderrBuf, err = runAndWait(suite, cmd)
	}

	if err != nil {
		// check that command failed, not something else happened
		_, ok := err.(*exec.ExitError)
		suite.Require().True(ok, "%s", err)
	}

	if stdoutBuf != nil {
		stdout = stdoutBuf.String()
	}

	var stderr string
	if stderrBuf != nil {
		stderr = stderrBuf.String()
	}

	if opts.shouldFail {
		suite.Assert().Error(err, "command expected to fail, but did not")
	} else {
		suite.Assert().NoError(err, "command failed")
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
