// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package testnetns runs tests in an isolated network namespace.
package testnetns

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const childEnvironment = "TALOS_TEST_NETNS_CHILD"

// Run is called from TestMain as os.Exit(testnetns.Run(m.Run)). It reexecutes
// the test binary with its original arguments and streams, so test selection,
// repetition, JSON output, skips, and failures remain owned by testing.
//
// All goroutines in the child share the new network namespace. Its loopback
// interface is initially down; callers must configure it if needed. Tests still
// share one namespace and must clean up their fixtures between cases.
// Creating the namespace requires CAP_SYS_ADMIN, as provided by privileged CI.
// Namespace creation failures fail the suite rather than silently losing tests.
func Run(run func() int) int {
	if os.Getenv(childEnvironment) == "1" {
		return run()
	}

	cmd := exec.CommandContext(context.Background(), os.Args[0], os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	cmd.Env = append(os.Environ(), childEnvironment+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWNET}

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() >= 0 {
			return exitErr.ExitCode()
		}

		fmt.Fprintf(os.Stderr, "testnetns: running tests in a network namespace: %v\n", err)

		return 1
	}

	return 0
}
