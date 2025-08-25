// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package fsopen

import (
	"maps"
	"strings"
)

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

// WithPrinter adds a printer to the filesystem configuration.
func WithPrinter(printer func(string, ...any)) Option {
	return Option{
		set: func(t *FS) {
			t.printer = printer
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

			t.boolParams["prjquota"] = struct{}{}
		},
	}
}

// WithStringParameter adds a map of strings to the filesystem configuration.
func WithStringParameter(param, value string) Option {
	return Option{
		set: func(t *FS) {
			if t.stringParams == nil {
				t.stringParams = make(map[string]string)
			}

			t.stringParams[param] = value
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
				t.binaryParams = make(map[string][]byte)
			}

			t.binaryParams[param] = value
		},
	}
}

// WithParameters parses parameters and adds them to the filesystem configuration.
func WithParameters(parameters string) Option {
	return Option{
		set: func(t *FS) {
			sparams, bparams := parseParameters(parameters)

			if t.stringParams == nil {
				t.stringParams = make(map[string]string)
			}

			maps.Insert(t.stringParams, maps.All(sparams))

			maps.Insert(t.boolParams, maps.All(bparams))
		},
	}
}

func parseParameters(parameters string) (map[string]string, map[string]struct{}) {
	if parameters == "" {
		return nil, nil
	}

	var (
		bparams = make(map[string]struct{})
		sparams = make(map[string]string)
	)

	for param := range strings.SplitSeq(parameters, ",") {
		if param == "" {
			continue
		}

		kv := strings.SplitN(param, "=", 2)

		if len(kv) == 2 {
			sparams[kv[0]] = kv[1]
		} else {
			bparams[kv[0]] = struct{}{}
		}
	}

	return sparams, bparams
}
