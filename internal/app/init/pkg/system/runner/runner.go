/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package runner

import (
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
)

// Runner describes the requirements for running a process.
type Runner interface {
	Run() error
	Stop() error
}

// Args represents the required options for services.
type Args struct {
	ID          string
	ProcessArgs []string
}

// Options is the functional options struct.
type Options struct {
	// Env describes the service's environment variables. Elements should be in
	// the format <key=<value>
	Env []string
	// ContainerOpts describes the container options.
	ContainerOpts []containerd.NewContainerOpts
	// OCISpecOpts describes the OCI spec options.
	OCISpecOpts []oci.SpecOpts
	// ContainerImage is the container's image.
	ContainerImage string
	// Namespace is the containerd namespace.
	Namespace string
	// Type describes the service's restart policy.
	Type Type
	// LogPath is the root path to store logs
	LogPath string
	// RestartInterval is the interval between restarts for failed runs
	RestartInterval time.Duration
	// GracefulShutdownTimeout is the time to wait for process to exit after SIGTERM
	// before sending SIGKILL
	GracefulShutdownTimeout time.Duration
}

// Option is the functional option func.
type Option func(*Options)

// Type represents the service's restart policy.
type Type int

const (
	// Forever will always restart a process.
	Forever Type = iota
	// Once will restart the process only if it did not exit successfully.
	Once
)

// DefaultOptions describes the default options to a runner.
func DefaultOptions() *Options {
	return &Options{
		Env:                     []string{},
		Type:                    Forever,
		Namespace:               "system",
		LogPath:                 "/var/log",
		RestartInterval:         5 * time.Second,
		GracefulShutdownTimeout: 10 * time.Second,
	}
}

// WithType sets the type of a service.
func WithType(o Type) Option {
	return func(args *Options) {
		args.Type = o
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

// WithLogPath sets the log path root
func WithLogPath(path string) Option {
	return func(args *Options) {
		args.LogPath = path
	}
}

// WithRestartInterval sets the interval between restarts of the failed task
func WithRestartInterval(interval time.Duration) Option {
	return func(args *Options) {
		args.RestartInterval = interval
	}
}

// WithGracefulShutdownTimeout sets the timeout for the task to terminate before sending SIGKILL
func WithGracefulShutdownTimeout(timeout time.Duration) Option {
	return func(args *Options) {
		args.GracefulShutdownTimeout = timeout
	}
}
