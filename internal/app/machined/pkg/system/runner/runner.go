// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package runner provides a runner for running services.
package runner

import (
	"fmt"
	"io"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Runner describes the requirements for running a process.
type Runner interface {
	fmt.Stringer
	Open() error
	Run(events.Recorder) error
	Stop() error
	Close() error
}

// Args represents the required options for services.
type Args struct {
	ID          string
	ProcessArgs []string
}

// Options is the functional options struct.
type Options struct {
	// LoggingManager provides service log handling.
	LoggingManager runtime.LoggingManager
	// Env describes the service's environment variables. Elements should be in
	// the format <key=<value>
	Env []string
	// ContainerdAddress is containerd socket address.
	ContainerdAddress string
	// ContainerOpts describes the container options.
	ContainerOpts []containerd.NewContainerOpts
	// OCISpecOpts describes the OCI spec options.
	OCISpecOpts []oci.SpecOpts
	// ContainerImage is the container's image.
	ContainerImage string
	// Namespace is the containerd namespace.
	Namespace string
	// GracefulShutdownTimeout is the time to wait for process to exit after SIGTERM
	// before sending SIGKILL
	GracefulShutdownTimeout time.Duration
	// Stdin is the process standard input.
	Stdin io.ReadSeeker
	// Specify an oom_score_adj for the process.
	OOMScoreAdj int
	// CgroupPath (optional) sets the cgroup path to use
	CgroupPath string
	// OverrideSeccompProfile default Linux seccomp profile.
	OverrideSeccompProfile func(*specs.LinuxSeccomp)
	// DroppedCapabilities is the list of capabilities to drop.
	DroppedCapabilities []string
	// StdinFile is the path to the file to use as stdin.
	StdinFile string
	// StdoutFile is the path to the file to use as stdout.
	StdoutFile string
	// StderrFile is the path to the file to use as stderr.
	StderrFile string
	// Ctty is the controlling tty.
	Ctty optional.Optional[int]
	// UID is the user id of the process.
	UID uint32
}

// Option is the functional option func.
type Option func(*Options)

// DefaultOptions describes the default options to a runner.
func DefaultOptions() *Options {
	return &Options{
		LoggingManager:          logging.NewNullLoggingManager(),
		Env:                     []string{},
		Namespace:               constants.SystemContainerdNamespace,
		GracefulShutdownTimeout: 10 * time.Second,
		ContainerdAddress:       constants.CRIContainerdAddress,
		Stdin:                   nil,
		OOMScoreAdj:             0,
	}
}

// WithEnv sets the environment variables of a service.
func WithEnv(o []string) Option {
	return func(args *Options) {
		args.Env = o
	}
}

// WithNamespace sets the tar file to load.
func WithNamespace(o string) Option {
	return func(args *Options) {
		args.Namespace = o
	}
}

// WithContainerdAddress sets the containerd socket path.
func WithContainerdAddress(a string) Option {
	return func(args *Options) {
		args.ContainerdAddress = a
	}
}

// WithContainerImage sets the image ref.
func WithContainerImage(o string) Option {
	return func(args *Options) {
		args.ContainerImage = o
	}
}

// WithContainerOpts sets the containerd container options.
func WithContainerOpts(o ...containerd.NewContainerOpts) Option {
	return func(args *Options) {
		args.ContainerOpts = o
	}
}

// WithOCISpecOpts sets the OCI spec options.
func WithOCISpecOpts(o ...oci.SpecOpts) Option {
	return func(args *Options) {
		args.OCISpecOpts = o
	}
}

// WithLoggingManager sets the LoggingManager option.
func WithLoggingManager(manager runtime.LoggingManager) Option {
	return func(args *Options) {
		args.LoggingManager = manager
	}
}

// WithGracefulShutdownTimeout sets the timeout for the task to terminate before sending SIGKILL.
func WithGracefulShutdownTimeout(timeout time.Duration) Option {
	return func(args *Options) {
		args.GracefulShutdownTimeout = timeout
	}
}

// WithStdin sets the standard input.
func WithStdin(stdin io.ReadSeeker) Option {
	return func(args *Options) {
		args.Stdin = stdin
	}
}

// WithOOMScoreAdj sets the oom_score_adj.
func WithOOMScoreAdj(score int) Option {
	return func(args *Options) {
		args.OOMScoreAdj = score
	}
}

// WithCgroupPath sets the cgroup path.
func WithCgroupPath(path string) Option {
	return func(args *Options) {
		args.CgroupPath = path
	}
}

// WithCustomSeccompProfile sets the function to override seccomp profile.
func WithCustomSeccompProfile(override func(*specs.LinuxSeccomp)) Option {
	return func(args *Options) {
		args.OverrideSeccompProfile = override
	}
}

// WithDroppedCapabilities sets the list of capabilities to drop.
func WithDroppedCapabilities(caps map[string]struct{}) Option {
	return func(args *Options) {
		args.DroppedCapabilities = maps.Keys(caps)
	}
}

// WithStdinFile sets the path to the file to use as stdin.
func WithStdinFile(path string) Option {
	return func(args *Options) {
		args.StdinFile = path
	}
}

// WithStdoutFile sets the path to the file to use as stdout.
func WithStdoutFile(path string) Option {
	return func(args *Options) {
		args.StdoutFile = path
	}
}

// WithStderrFile sets the path to the file to use as stderr.
func WithStderrFile(path string) Option {
	return func(args *Options) {
		args.StdoutFile = path
	}
}

// WithCtty sets the controlling tty.
func WithCtty(ctty int) Option {
	return func(args *Options) {
		args.Ctty = optional.Some(ctty)
	}
}

// WithUID sets the user id of the process.
func WithUID(uid uint32) Option {
	return func(args *Options) {
		args.UID = uid
	}
}
