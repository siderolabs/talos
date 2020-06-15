// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package argsbuilder

import "fmt"

// Key represents an arg key.
type Key = string

// Value represents an arg value.
type Value = string

// Args represents a set of args.
type Args map[Key]Value

// Merge implements the ArgsBuilder interface.
func (a Args) Merge(args Args) ArgsBuilder {
	for key, val := range args {
		a[key] = val
	}

	return a
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
