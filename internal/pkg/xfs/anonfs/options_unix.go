// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package anonfs

import "maps"

// Option is a functional option for configuring a filesystem instance.
type Option struct {
	set func(*AnonFS) error
}

// WithStrings adds a map of strings to the filesystem configuration.
func WithStrings(strings map[string]string) Option {
	return Option{
		set: func(t *AnonFS) error {
			maps.Insert(t.strings, maps.All(strings))

			return nil
		},
	}
}

// WithFlags adds a list of flags to the filesystem configuration.
func WithFlags(flags ...string) Option {
	return Option{
		set: func(t *AnonFS) error {
			t.flags = append(t.flags, flags...)

			return nil
		},
	}
}
