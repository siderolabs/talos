// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package argsbuilder

import (
	"fmt"
	"strings"
)

// Key represents an arg key.
type Key = string

// Value represents an arg value.
type Value = string

// Args represents a set of args.
type Args map[Key]Value

// MustMerge implements the ArgsBuilder interface.
func (a Args) MustMerge(args Args, setters ...MergeOption) {
	if err := a.Merge(args, setters...); err != nil {
		panic(err)
	}
}

// Merge implements the ArgsBuilder interface.
//nolint:gocyclo
func (a Args) Merge(args Args, setters ...MergeOption) error {
	var opts MergeOptions

	for _, s := range setters {
		s(&opts)
	}

	policies := opts.Policies
	if policies == nil {
		policies = MergePolicies{}
	}

	for key, val := range args {
		policy := policies[key]

		switch policy {
		case MergeDenied:
			return NewDenylistError(key)
		case MergeAdditive:
			values := strings.Split(a[key], ",")
			definedValues := map[string]struct{}{}

			for i, v := range values {
				definedValues[strings.TrimSpace(v)] = struct{}{}

				if v == "" {
					values = append(values[:i], values[i+1:]...)
				}
			}

			for _, v := range strings.Split(val, ",") {
				v = strings.TrimSpace(v)
				if _, defined := definedValues[v]; !defined {
					values = append(values, v)
				}
			}

			a[key] = strings.Join(values, ",")
		case MergeOverwrite:
			a[key] = val
		}
	}

	return nil
}

// Set implements the ArgsBuilder interface.
func (a Args) Set(k, v Key) ArgsBuilder {
	a[k] = v

	return a
}

// Args implements the ArgsBuilder interface.
func (a Args) Args() []string {
	args := []string{}

	for key, val := range a {
		args = append(args, fmt.Sprintf("--%s=%s", key, val))
	}

	return args
}

// Get returns an args value.
func (a Args) Get(k Key) Value {
	return a[k]
}

// Contains checks if an arg key exists.
func (a Args) Contains(k Key) bool {
	_, ok := a[k]

	return ok
}

// DenyListError represents an error indicating that an argument was supplied
// that is not allowed.
type DenyListError struct {
	s string
}

// NewDenylistError returns a DenyListError.
func NewDenylistError(s string) error {
	return &DenyListError{s}
}

// Error implements the Error interface.
func (b *DenyListError) Error() string {
	return fmt.Sprintf("extra arg %q is not allowed", b.s)
}
