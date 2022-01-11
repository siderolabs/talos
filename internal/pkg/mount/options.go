// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import "github.com/talos-systems/talos/pkg/machinery/config"

const (
	// ReadOnly is a flag for setting the mount point as readonly.
	ReadOnly Flags = 1 << iota
	// Shared is a flag for setting the mount point as shared.
	Shared
	// Resize indicates that a the partition for a given mount point should be
	// resized to the maximum allowed.
	Resize
	// Overlay indicates that a the partition for a given mount point should be
	// mounted using overlayfs.
	Overlay
	// ReadonlyOverlay indicates that a the partition for a given mount point should be
	// mounted using multi-layer readonly overlay from multiple partitions given as sources.
	ReadonlyOverlay
	// SkipIfMounted is a flag for skipping mount if the mountpoint is already mounted.
	SkipIfMounted
	// SkipIfNoFilesystem is a flag for skipping formatting and mounting if the mountpoint has not filesystem.
	SkipIfNoFilesystem
)

// Flags is the mount flags.
type Flags uint

// Options is the functional options struct.
type Options struct {
	Loopback         string
	Prefix           string
	MountFlags       Flags
	PreMountHooks    []Hook
	PostUnmountHooks []Hook
	Encryption       config.Encryption
}

// Option is the functional option func.
type Option func(*Options)

// Check checks if all provided flags are set.
func (f Flags) Check(flags Flags) bool {
	return (f & flags) == flags
}

// Intersects checks if at least one flag is set.
func (f Flags) Intersects(flags Flags) bool {
	return (f & flags) != 0
}

// WithPrefix is a functional option for setting the mount point prefix.
func WithPrefix(o string) Option {
	return func(args *Options) {
		args.Prefix = o
	}
}

// WithFlags is a functional option to set up mount flags.
func WithFlags(flags Flags) Option {
	return func(args *Options) {
		args.MountFlags = flags
	}
}

// WithPreMountHooks adds functions to be called before mounting the partition.
func WithPreMountHooks(hooks ...Hook) Option {
	return func(args *Options) {
		args.PreMountHooks = hooks
	}
}

// WithPostUnmountHooks adds functions to be called after unmounting the partition.
func WithPostUnmountHooks(hooks ...Hook) Option {
	return func(args *Options) {
		args.PostUnmountHooks = hooks
	}
}

// WithEncryptionConfig partition encryption configuration.
func WithEncryptionConfig(cfg config.Encryption) Option {
	return func(args *Options) {
		args.Encryption = cfg
	}
}

// Hook represents pre/post mount hook.
type Hook func(p *Point) error

// NewDefaultOptions initializes a Options struct with default values.
func NewDefaultOptions(setters ...Option) *Options {
	opts := &Options{
		Loopback:         "",
		Prefix:           "",
		MountFlags:       0,
		PreMountHooks:    []Hook{},
		PostUnmountHooks: []Hook{},
	}

	for _, setter := range setters {
		setter(opts)
	}

	return opts
}
