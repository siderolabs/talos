// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package runner provides a runner for running services.
package runner

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/containerd/cgroups/v3/cgroup2"
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
	// Run runs the process to completion and reports how it ended.
	//
	// Canceling ctx asks the process to stop gracefully, and Run returns once it has. A Runner may be
	// run more than once, which is what lets the restart wrapper drive it.
	Run(ctx context.Context, eventSink events.Recorder, onStart OnStart) (Status, error)
	Close() error
}

// OnStart is a callback, called with the process PID when it's started.
type OnStart func(pid int32)

// Status is how a single run ended.
type Status struct {
	// Started reports whether the process ever ran. When it is false the run failed before that
	// point, which is a different thing from a process that ran and exited.
	Started bool
	// ExitCode is the process's exit code, and is meaningful only when Started.
	//
	// A runner which cannot observe a numeric exit code reports zero and describes the failure in
	// its error instead.
	ExitCode int
}

// Args represents the required options for services.
type Args struct {
	ID          string
	ProcessArgs []string
}

// IOPriorityParam represents the combination of IO scheduling class and priority.
type IOPriorityParam struct {
	Class    uint
	Priority uint
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
	// CgroupResources (optional) sets the resource limits for the cgroup; when nil they come from
	// the built-in table keyed by cgroup name.
	CgroupResources *cgroup2.Resources
	// LogID (optional) is the identifier the process's log is registered under; defaults to the
	// runner's ID.
	LogID string
	// HostNetworkFiles mounts the host's /etc/hosts and /etc/resolv.conf into the container.
	HostNetworkFiles bool
	// OverrideSeccompProfile default Linux seccomp profile.
	OverrideSeccompProfile func(*specs.LinuxSeccomp)
	// DroppedCapabilities is the list of capabilities to drop.
	DroppedCapabilities []string
	// SelinuxLabel is the SELinux label to be assigned
	SelinuxLabel string
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
	// Priority is the niceness value of the process.
	Priority int
	// IOPriority is the IO priority value and class of the process.
	IOPriority optional.Optional[IOPriorityParam]
	// SchedulingPolicy is the scheduling policy of the process.
	SchedulingPolicy optional.Optional[uint]
	// Sandbox, when non-nil, returns the launcher for the shared sandbox
	// PID+mount namespace; the process is launched inside it instead of the host
	// namespace. It is evaluated at each (re)launch so a recreated namespace is
	// picked up; a nil return means the namespace is not currently available.
	Sandbox func() runtime.SandboxLauncher
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
		HostNetworkFiles:        true,
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

// WithCgroupResources sets the resource limits for the cgroup.
//
// Without this the limits come from the built-in table keyed by cgroup name, which is what the Talos
// services want; a caller running arbitrary workloads has limits of its own to apply.
func WithCgroupResources(resources *cgroup2.Resources) Option {
	return func(args *Options) {
		args.CgroupResources = resources
	}
}

// WithLogID sets the identifier the process's log is registered under.
//
// It defaults to the runner's ID. Setting it apart matters when the process ID is not the identity
// the logs should follow, e.g. one container restarted as a succession of differently-named
// instances whose output belongs in one place.
func WithLogID(id string) Option {
	return func(args *Options) {
		args.LogID = id
	}
}

// WithHostNetworkFiles controls whether the host's /etc/hosts and /etc/resolv.conf are mounted in.
//
// On by default, which is what a service sharing the host network wants; a container with a network
// namespace of its own has no business seeing them.
func WithHostNetworkFiles(enabled bool) Option {
	return func(args *Options) {
		args.HostNetworkFiles = enabled
	}
}

// WithSelinuxLabel sets the SELinux label.
func WithSelinuxLabel(label string) Option {
	return func(args *Options) {
		args.SelinuxLabel = label
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

// WithPriority sets the priority of the process.
func WithPriority(priority int) Option {
	return func(args *Options) {
		args.Priority = priority
	}
}

const (
	// IoprioClassNone represents IOPRIO_CLASS_NONE.
	IoprioClassNone = iota
	// IoprioClassRt represents IOPRIO_CLASS_RT.
	IoprioClassRt
	// IoprioClassBe represents IOPRIO_CLASS_BE.
	IoprioClassBe
	// IoprioClassIdle represents IOPRIO_CLASS_IDLE.
	IoprioClassIdle
)

// WithIOPriority sets the IO priority and class of the process.
func WithIOPriority(class, priority uint) Option {
	return func(args *Options) {
		args.IOPriority = optional.Some(IOPriorityParam{
			Class:    class,
			Priority: priority,
		})
	}
}

const (
	// SchedulingPolicyNormal represents SCHED_NORMAL.
	SchedulingPolicyNormal = iota
	// SchedulingPolicyFIFO represents SCHED_FIFO.
	SchedulingPolicyFIFO
	// SchedulingPolicyRR represents SCHED_RR.
	SchedulingPolicyRR
	// SchedulingPolicyBatch represents SCHED_BATCH.
	SchedulingPolicyBatch
	// SchedulingPolicyIsoUnimplemented represents SCHED_ISO.
	SchedulingPolicyIsoUnimplemented
	// SchedulingPolicyIdle represents SCHED_IDLE.
	SchedulingPolicyIdle
	// SchedulingPolicyDeadline represents SCHED_DEADLINE.
	SchedulingPolicyDeadline
)

// WithSchedulingPolicy sets the scheduling policy of the process.
func WithSchedulingPolicy(policy uint) Option {
	return func(args *Options) {
		args.SchedulingPolicy = optional.Some(policy)
	}
}

// WithSandbox causes the process to be launched inside the shared
// sandbox PID+mount namespace. The getter is evaluated at each launch; a nil
// getter is a no-op (host namespace), and a getter returning nil means the
// namespace is not yet available (the launch is retried).
func WithSandbox(launcher func() runtime.SandboxLauncher) Option {
	return func(args *Options) {
		args.Sandbox = launcher
	}
}
