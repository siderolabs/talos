// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package fsopen

// Option is a functional option for configuring a filesystem instance.
type Option struct {
	set func(*FS)
}

// WithSource adds a source to the filesystem configuration.
func WithSource(source string) Option {
	return Option{
		set: func(t *FS) {
			t.source = source
		},
	}
}

// WithMountFlags adds a flag set that will be passed to Fsmount syscall.
func WithMountFlags(flag int) Option {
	return Option{
		set: func(f *FS) {
			f.mntflags |= flag
		},
	}
}

// WithProjectQuota sets the project quota flag.
func WithProjectQuota(enabled bool) Option {
	return Option{
		set: func(t *FS) {
			if !enabled {
				return
			}

			if t.boolParams == nil {
				t.boolParams = make(map[string]struct{})
			}

			t.boolParams["prjquota"] = struct{}{}
		},
	}
}

// WithStringParameter adds a map of strings to the filesystem configuration.
func WithStringParameter(param, value string) Option {
	return Option{
		set: func(t *FS) {
			if t.stringParams == nil {
				t.stringParams = make(map[string][]string)
			}

			t.stringParams[param] = append(t.stringParams[param], value)
		},
	}
}

// WithBoolParameter adds a flag parameter to the filesystem configuration.
func WithBoolParameter(param string) Option {
	return Option{
		set: func(t *FS) {
			if t.boolParams == nil {
				t.boolParams = make(map[string]struct{})
			}

			t.boolParams[param] = struct{}{}
		},
	}
}

// WithBinaryParameters adds a map of byte arrays to the filesystem configuration.
func WithBinaryParameters(param string, value []byte) Option {
	return Option{
		set: func(t *FS) {
			if t.binaryParams == nil {
				t.binaryParams = make(map[string][][]byte)
			}

			t.binaryParams[param] = append(t.binaryParams[param], value)
		},
	}
}
