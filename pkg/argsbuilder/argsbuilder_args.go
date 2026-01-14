// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package argsbuilder

import (
	"fmt"
	"slices"
	"strings"

	"github.com/siderolabs/gen/maps"
)

// Key represents an arg key.
type Key = string

// Value represents an arg value.
type Value = []string

// Args represents a set of args.
type Args map[Key]Value

// MustMerge implements the ArgsBuilder interface.
func (a Args) MustMerge(args Args, setters ...MergeOption) {
	if err := a.Merge(args, setters...); err != nil {
		panic(err)
	}
}

// Merge implements the ArgsBuilder interface.
//
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

		case MergeOverwrite:
			a[key] = slices.Clone(val)

		case MergeAppend:
			existing := make([]string, 0, len(val)+len(a[key]))
			existing = append(existing, a[key]...)
			existing = append(existing, val...)
			a[key] = existing

		case MergePrepend:
			existing := make([]string, 0, len(val)+len(a[key]))
			existing = append(existing, val...)
			existing = append(existing, a[key]...)
			a[key] = existing

		case MergeAdditive:
			// 1. Join the existing []string slice into one string so we can Split it.
			//    This handles cases where a[key] might be ["a", "b"] or ["a,b"].
			rawExisting := strings.Join(a[key], ",")
			values := strings.Split(rawExisting, ",")

			definedValues := map[string]struct{}{}
			i := 0

			for _, v := range values {
				cleanV := strings.TrimSpace(v)
				if cleanV != "" {
					// Only keep if unique
					if _, seen := definedValues[cleanV]; !seen {
						definedValues[cleanV] = struct{}{}
						values[i] = cleanV
						i++
					}
				}
			}

			values = values[:i]

			// 2. Join the incoming 'val' slice so we can SplitSeq over it.
			rawIncoming := strings.Join(val, ",")

			for v := range strings.SplitSeq(rawIncoming, ",") {
				v = strings.TrimSpace(v)
				if v != "" {
					if _, defined := definedValues[v]; !defined {
						values = append(values, v)
						// Mark as defined to prevent duplicates within the incoming values too
						definedValues[v] = struct{}{}
					}
				}
			}

			// 3. Join the results and wrap in a []string to satisfy the type.
			a[key] = []string{strings.Join(values, ",")}
		}
	}

	return nil
}

// Set implements the ArgsBuilder interface.
func (a Args) Set(k Key, v Value) ArgsBuilder {
	a[k] = v

	return a
}

// Args implements the ArgsBuilder interface.
func (a Args) Args() []string {
	keys := maps.Keys(a)
	slices.Sort(keys)

	args := make([]string, 0, len(a))

	for _, key := range keys {
		vals := a[key]

		for _, val := range vals {
			args = append(
				args,
				fmt.Sprintf("--%s=%s", key, val),
			)
		}
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
