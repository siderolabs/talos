// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package makefs provides function to format and grow filesystems.
package makefs

import (
	"crypto/sha256"

	"github.com/google/uuid"
)

// Option to control makefs settings.
type Option func(*Options)

// Options for makefs.
type Options struct {
	Label                  string
	ConfigFile             string
	SourceDirectory        string
	SectorSize             uint
	DeviceSize             uint64
	MinAllocationGroupSize uint64
	Force                  bool
	Reproducible           bool
	UnsupportedFSOption    bool

	Printf func(string, ...any)
}

// WithLabel sets the label for the filesystem to be created.
func WithLabel(label string) Option {
	return func(o *Options) {
		o.Label = label
	}
}

// WithForce forces creation of a filesystem even if one already exists.
func WithForce(force bool) Option {
	return func(o *Options) {
		o.Force = force
	}
}

// WithReproducible sets the reproducible flag for the filesystem to be created.
// This should only be used when creating filesystems on raw disk images.
func WithReproducible(reproducible bool) Option {
	return func(o *Options) {
		o.Reproducible = reproducible
	}
}

// WithUnsupportedFSOption sets the unsupported filesystem option.
func WithUnsupportedFSOption(unsupported bool) Option {
	return func(o *Options) {
		o.UnsupportedFSOption = unsupported
	}
}

// WithConfigFile sets the config file for the filesystem to be created.
func WithConfigFile(configFile string) Option {
	return func(o *Options) {
		o.ConfigFile = configFile
	}
}

// WithSourceDirectory sets the source directory for populating the filesystem.
func WithSourceDirectory(sourceDir string) Option {
	return func(o *Options) {
		o.SourceDirectory = sourceDir
	}
}

// WithSectorSize overrides the sector size used by mkfs. This should only be
// used with disk images where the underlying sector size cannot be detected
// automatically; on real block devices mkfs auto-detection is preferred.
//
// For ext4, this sets the filesystem block size (-b) since ext4 has no
// separate sector size concept.
func WithSectorSize(sectorSize uint) Option {
	return func(o *Options) {
		o.SectorSize = sectorSize
	}
}

// WithDeviceSize sets the size of the device being formatted, in bytes.
//
// It is only used to derive filesystem geometry, see WithMinAllocationGroupSize.
func WithDeviceSize(size uint64) Option {
	return func(o *Options) {
		o.DeviceSize = size
	}
}

// WithMinAllocationGroupSize sets the minimum allocation group size (in bytes) for XFS.
//
// It has no effect unless WithDeviceSize is set as well. Zero leaves the mkfs defaults alone.
func WithMinAllocationGroupSize(size uint64) Option {
	return func(o *Options) {
		o.MinAllocationGroupSize = size
	}
}

// WithPrintf sets the printf function for logging.
func WithPrintf(printf func(string, ...any)) Option {
	return func(o *Options) {
		o.Printf = printf
	}
}

// NewDefaultOptions builds options with specified setters applied.
func NewDefaultOptions(setters ...Option) Options {
	var opt Options

	for _, o := range setters {
		o(&opt)
	}

	if opt.Printf == nil {
		opt.Printf = func(string, ...any) {}
	}

	return opt
}

// GUIDFromLabel generates a deterministic partition GUID from a label by
// creating a version 8 UUID derived from a SHA-256 hash of the label bytes.
func GUIDFromLabel(label string) uuid.UUID {
	// version 8 UUID since we're doing custom hashing
	return uuid.NewHash(sha256.New(), uuid.Nil, []byte(label), 8)
}
