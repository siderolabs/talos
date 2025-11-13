// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package base

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
func runAndWait(t *testing.T, cmd *exec.Cmd) (stdoutBuf, stderrBuf *bytes.Buffer, err error) {
	var stdout, stderr bytes.Buffer

	cmd.Stdin = nil
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = []string{}

	// filter environment variables
	for _, keyvalue := range os.Environ() {
		name, _, ok := strings.Cut(keyvalue, "=")
		if !ok {
			continue
		}

		switch strings.ToUpper(name) {
		case "PATH":
			fallthrough
		case "HOME":
			fallthrough
		case "USERNAME":
			cmd.Env = append(cmd.Env, keyvalue)
		}
	}

	t.Logf("Running %q", strings.Join(cmd.Args, " "))

	require.NoError(t, cmd.Start())

	err = cmd.Wait()

	return &stdout, &stderr, err
}

// retryRunAndWait retries runAndWait if the command fails to run.
func retryRunAndWait(t *testing.T, cmdFunc func() *exec.Cmd, retryer retry.Retryer) (stdoutBuf, stderrBuf *bytes.Buffer, err error) {
	err = retryer.Retry(func() error {
		stdoutBuf, stderrBuf, err = runAndWait(t, cmdFunc())

		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return retry.ExpectedErrorf("command failed, stderr %v: %w", stderrBuf.String(), err)
		}

		return err
	})

	return stdoutBuf, stderrBuf, err
}

// run executes command, asserts on its exit status/output, and returns stdout.
//
//nolint:gocyclo,nakedret
func run(t *testing.T, cmdFunc func() *exec.Cmd, options ...RunOption) (stdout, stderr string) {
	var opts runOptions

	for _, o := range options {
		o(&opts)
	}

	var (
		stdoutBuf, stderrBuf *bytes.Buffer
		err                  error
	)

	if opts.retryer != nil {
		stdoutBuf, stderrBuf, err = retryRunAndWait(t, cmdFunc, opts.retryer)
	} else {
		stdoutBuf, stderrBuf, err = runAndWait(t, cmdFunc())
	}

	if err != nil {
		// check that command failed, not something else happened
		var exitError *exec.ExitError

		require.True(t, errors.As(err, &exitError), "%s", err)
	}

	if stdoutBuf != nil {
		stdout = stdoutBuf.String()
	}

	if stderrBuf != nil {
		stderr = stderrBuf.String()
	}

	if opts.shouldFail {
		assert.Error(t, err, "command expected to fail, but did not")
	} else {
		assert.NoError(t, err, "command failed, stdout: %q, stderr: %q", stdout, stderr)
	}

	if opts.stdoutEmpty {
		assert.Empty(t, stdout, "stdout should be empty")
	} else {
		assert.NotEmpty(t, stdout, "stdout should be not empty")
	}

	if opts.stderrNotEmpty {
		assert.NotEmpty(t, stderr, "stderr should be not empty")
	} else {
		assert.Empty(t, stderr, "stderr should be empty")
	}

	for _, rx := range opts.stdoutRegexps {
		assert.Regexp(t, rx, stdout)
	}

	for _, rx := range opts.stderrRegexps {
		assert.Regexp(t, rx, stderr)
	}

	for _, rx := range opts.stdoutNegativeRegexps {
		assert.NotRegexp(t, rx, stdout)
	}

	for _, rx := range opts.stderrNegativeRegexps {
		assert.NotRegexp(t, rx, stderr)
	}

	for _, f := range opts.stdoutMatchers {
		assert.NoError(t, f(stdout), "stdout match: %q", stdout)
	}

	for _, f := range opts.stderrMatchers {
		assert.NoError(t, f(stderr), "stderr match: %q", stderr)
	}

	return stdout, stderr
}
