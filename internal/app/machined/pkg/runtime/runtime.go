// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"os"
	"syscall"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// SandboxHandle is a handle to a service process launched inside the sandbox
// PID namespace.
type SandboxHandle interface {
	// HostPID returns the process PID as seen from the host (root) PID namespace.
	HostPID() int
	// Signal sends a signal to the service process.
	Signal(sig syscall.Signal) error
	// Wait waits for the service process to exit and returns the exit code.
	Wait() (int, error)
	// Close releases resources associated with the service handle.
	Close() error
}

// LaunchConfig carries everything needed to launch a service in the sandbox
// namespace.
type LaunchConfig struct {
	Args                []string
	Env                 []string
	DroppedCapabilities []string
	SelinuxLabel        string
	// Stdin is the file to use as the service's stdin.  When nil, /dev/null is used.
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
}

// SandboxLauncher launches service processes inside the sandbox PID+mount
// namespace managed by sandboxd.
type SandboxLauncher interface {
	// Launch forks+execs the service inside the sandbox namespace.
	// The caller is responsible for closing its own copies of the fds after
	// Launch returns.
	Launch(cfg LaunchConfig) (SandboxHandle, error)
}

// Runtime defines the runtime parameters.
type Runtime interface { //nolint:interfacebloat
	Config() config.Config
	ConfigContainer() config.Container
	ConfigCompleteForBoot() bool
	RollbackToConfigAfter(time.Duration) error
	CancelConfigRollbackTimeout()
	SetConfig(config.Provider) error
	SetPersistedConfig(config.Provider) error
	State() State
	Events() EventStream
	Logging() LoggingManager
	NodeName() (string, error)
	IsBootstrapAllowed() bool
	GetSystemInformation(ctx context.Context) (*hardware.SystemInformation, error)
	// Sandbox returns the launcher for the shared sandbox PID+mount namespace.
	// Returns nil until sandboxd is running (and again while it is being
	// recreated after an unexpected exit).
	Sandbox() SandboxLauncher
	// SetSandbox publishes (or clears, with nil) the sandbox namespace launcher.
	// Called by the sandboxd service runner as the namespace is (re)created/torn down.
	SetSandbox(SandboxLauncher)
}
