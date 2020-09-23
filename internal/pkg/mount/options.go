// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

// Options is the functional options struct.
type Options struct {
	Loopback      string
	Prefix        string
	ReadOnly      bool
	Shared        bool
	Resize        bool
	Overlay       bool
	SkipIfMounted bool
}

// Option is the functional option func.
type Option func(*Options)

// WithPrefix is a functional option for setting the mount point prefix.
func WithPrefix(o string) Option {
	return func(args *Options) {
		args.Prefix = o
	}
}

// WithReadOnly is a functional option for setting the mount point as readonly.
func WithReadOnly(o bool) Option {
	return func(args *Options) {
		args.ReadOnly = o
	}
}

// WithShared is a functional option for setting the mount point as shared.
func WithShared(o bool) Option {
	return func(args *Options) {
		args.Shared = o
	}
}

// WithSkipIfMounted is a functional option for skipping mount if the mountpoint is already mounted.
func WithSkipIfMounted(o bool) Option {
	return func(args *Options) {
		args.SkipIfMounted = o
	}
}

// WithResize indicates that a the partition for a given mount point should be
// resized to the maximum allowed.
func WithResize(o bool) Option {
	return func(args *Options) {
		args.Resize = o
	}
}

// WithOverlay indicates that a the partition for a given mount point should be
// mounted using overlayfs.
func WithOverlay(o bool) Option {
	return func(args *Options) {
		args.Overlay = o
	}
}

// NewDefaultOptions initializes a Options struct with default values.
func NewDefaultOptions(setters ...Option) *Options {
	opts := &Options{
		Loopback: "",
		Prefix:   "",
		ReadOnly: false,
		Shared:   false,
		Resize:   false,
		Overlay:  false,
	}

	for _, setter := range setters {
		setter(opts)
	}

	return opts
}
