/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"

	"github.com/talos-systems/talos/internal/app/init/pkg/system/events"
)

// Runner describes the requirements for running a process.
type Runner interface {
	fmt.Stringer
	Open(ctx context.Context) error
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
	// LogPath is the root path to store logs
	LogPath string
	// GracefulShutdownTimeout is the time to wait for process to exit after SIGTERM
	// before sending SIGKILL
	GracefulShutdownTimeout time.Duration
}

// Option is the functional option func.
type Option func(*Options)

// DefaultOptions describes the default options to a runner.
func DefaultOptions() *Options {
	return &Options{
		Env:                     []string{},
		Namespace:               "system",
		LogPath:                 "/var/log",
		GracefulShutdownTimeout: 10 * time.Second,
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

// WithGracefulShutdownTimeout sets the timeout for the task to terminate before sending SIGKILL
func WithGracefulShutdownTimeout(timeout time.Duration) Option {
	return func(args *Options) {
		args.GracefulShutdownTimeout = timeout
	}
}
