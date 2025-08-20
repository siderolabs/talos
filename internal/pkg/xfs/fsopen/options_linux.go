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
	set func(*FS) error
}

// WithStringParameters adds a map of strings to the filesystem configuration.
func WithStringParameters(flags map[string]string) Option {
	return Option{
		set: func(t *FS) error {
			if t.stringParams == nil {
				t.stringParams = make(map[string]string)
			}

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

// WithBinaryParameters adds a map of byte arrays to the filesystem configuration.
func WithBinaryParameters(params map[string][]byte) Option {
	return Option{
		set: func(t *FS) error {
			if t.binaryParams == nil {
				t.binaryParams = make(map[string][]byte)
			}

			maps.Insert(t.binaryParams, maps.All(params))

			return nil
		},
	}
}

// WithParameters parses parameters and adds them to the filesystem configuration.
func WithParameters(parameters string) Option {
	return Option{
		set: func(t *FS) error {
			sparams, bparams := parseParameters(parameters)

			if t.stringParams == nil {
				t.stringParams = make(map[string]string)
			}

			maps.Insert(t.stringParams, maps.All(sparams))

			t.boolParams = append(t.boolParams, bparams...)

			return nil
		},
	}
}

func parseParameters(parameters string) (map[string]string, []string) {
	if parameters == "" {
		return nil, nil
	}

	var (
		bparams []string
		sparams = make(map[string]string)
	)

	for param := range strings.SplitSeq(parameters, ",") {
		if param == "" {
			continue
		}

		kv := strings.SplitN(param, "=", 2)

		if len(kv) == 2 {
			bparams = append(bparams, kv[0])
		} else {
			sparams[kv[0]] = "true"
		}
	}

	return sparams, bparams
}
