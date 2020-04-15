// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package partition

import (
	"github.com/google/uuid"
)

// Options is the functional options struct.
type Options struct {
	Type        uuid.UUID
	Name        string
	MaximumSize bool
	Flags       uint64
}

// Option is the functional option func.
type Option func(*Options)

// WithPartitionType sets the partition type.
func WithPartitionType(id string) Option {
	return func(args *Options) {
		// TODO: An Option should return an error.
		// nolint: errcheck
		guuid, _ := uuid.Parse(id)
		args.Type = guuid
	}
}

// WithPartitionName sets the partition name.
func WithPartitionName(o string) Option {
	return func(args *Options) {
		args.Name = o
	}
}

// WithMaximumSize indicates if the partition should be created with the maximum size possible.
func WithMaximumSize(o bool) Option {
	return func(args *Options) {
		args.MaximumSize = o
	}
}

// WithLegacyBIOSBootableAttribute marks the partition as bootable.
func WithLegacyBIOSBootableAttribute(o bool) Option {
	return func(args *Options) {
		if o {
			args.Flags = 4
		}
	}
}

// NewDefaultOptions initializes a Options struct with default values.
func NewDefaultOptions(setters ...interface{}) *Options {
	// Default to data type "af3dc60f-8384-7247-8e79-3d69d8477de4"
	// TODO: An Option should return an error.
	// nolint: errcheck
	guuid, _ := uuid.Parse("af3dc60f-8384-7247-8e79-3d69d8477de4")

	opts := &Options{
		Type: guuid,
		Name: "",
	}

	for _, setter := range setters {
		if s, ok := setter.(Option); ok {
			s(opts)
		}
	}

	return opts
}
