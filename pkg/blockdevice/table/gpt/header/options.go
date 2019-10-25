// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package header

// Options is the functional options struct.
type Options struct {
	Primary bool
	Table   []byte
	Array   []byte
}

// Option is the functional option func.
type Option func(*Options)

// WithHeaderPrimary sets the primary option.
func WithHeaderPrimary(o bool) Option {
	return func(args *Options) {
		args.Primary = o
	}
}

// WithHeaderTable sets the partition type.
func WithHeaderTable(o []byte) Option {
	return func(args *Options) {
		args.Table = o
	}
}

// WithHeaderArrayBytes sets the partition type.
func WithHeaderArrayBytes(o []byte) Option {
	return func(args *Options) {
		args.Array = o
	}
}

// NewDefaultOptions initializes a Options struct with default values.
func NewDefaultOptions(setters ...interface{}) *Options {
	opts := &Options{
		Primary: true,
		Table:   []byte{},
		Array:   []byte{},
	}

	for _, setter := range setters {
		if s, ok := setter.(Option); ok {
			s(opts)
		}
	}

	return opts
}
