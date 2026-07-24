// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"slices"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Option is a functional option.
type Option func(o *Options) error

// Options describes the install options.
type Options struct {
	// Deprecated: Pull is not used in new Lifecycle API.
	Pull              bool
	Force             bool
	Upgrade           bool
	Zero              bool
	GrubUseUKICmdline bool
	ExtraKernelArgs   []string
}

// DefaultInstallOptions returns default options.
func DefaultInstallOptions() Options {
	return Options{}
}

// Apply list of Option.
func (o *Options) Apply(opts ...Option) error {
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return err
		}
	}

	return nil
}

// Environment returns the installer environment derived from the options.
func (o *Options) Environment(env []string) []string {
	env = slices.DeleteFunc(slices.Clone(env), func(value string) bool {
		name, _, _ := strings.Cut(value, "=")

		return name == constants.InstallerGrubUseUKICmdlineEnvVar
	})

	if o.GrubUseUKICmdline {
		env = append(env, constants.InstallerGrubUseUKICmdlineEnvVar+"=true")
	}

	return env
}

// WithOptions sets Options as a whole.
func WithOptions(opts Options) Option {
	return func(o *Options) error {
		*o = opts

		return nil
	}
}

// WithPull sets the pull option.
//
// Deprecated: Pull is not used in new Lifecycle API.
func WithPull(b bool) Option {
	return func(o *Options) error {
		o.Pull = b

		return nil
	}
}

// WithForce sets the force option.
func WithForce(b bool) Option {
	return func(o *Options) error {
		o.Force = b

		return nil
	}
}

// WithUpgrade sets the upgrade option.
func WithUpgrade(b bool) Option {
	return func(o *Options) error {
		o.Upgrade = b

		return nil
	}
}

// WithZero sets the zero option.
func WithZero(b bool) Option {
	return func(o *Options) error {
		o.Zero = b

		return nil
	}
}

// WithGrubUseUKICmdline configures GRUB to use the kernel command line embedded in the UKI.
func WithGrubUseUKICmdline(enabled bool) Option {
	return func(o *Options) error {
		o.GrubUseUKICmdline = enabled

		return nil
	}
}

// WithExtraKernelArgs sets the extra args.
func WithExtraKernelArgs(s []string) Option {
	return func(o *Options) error {
		o.ExtraKernelArgs = s

		return nil
	}
}
