// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package fsopen

import "maps"

// Option is a functional option for configuring a filesystem instance.
type Option struct {
	set func(*FS) error
}

// WithStringParameters adds a map of strings to the filesystem configuration.
func WithStringParameters(flags map[string]string) Option {
	return Option{
		set: func(t *FS) error {
			maps.Insert(t.stringParams, maps.All(flags))

			return nil
		},
	}
}

// WithBoolParameters adds a list of parameters to the filesystem configuration.
// Set the parameter named by the parameter to true.
func WithBoolParameters(params ...string) Option {
	return Option{
		set: func(t *FS) error {
			t.boolParams = append(t.boolParams, params...)

			return nil
		},
	}
}

// WithStrinWithBinaryParameters adds a map of byte arrays to the filesystem configuration.
func WithBinaryParameters(params map[string][]byte) Option {
	return Option{
		set: func(t *FS) error {
			maps.Insert(t.binaryParams, maps.All(params))

			return nil
		},
	}
}
