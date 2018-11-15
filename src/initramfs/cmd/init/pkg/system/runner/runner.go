package runner

import (
	"github.com/autonomy/talos/src/initramfs/pkg/userdata"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
)

// Runner describes the requirements for running a process.
type Runner interface {
	Run(*userdata.UserData, *Args, ...Option)
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
	// Type describes the service's restart policy.
	Type Type
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
		Env:  []string{},
		Type: Forever,
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
