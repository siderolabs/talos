// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package sliceutil contains utility functions for slices.
package sliceutil

import "gopkg.in/typ.v4/slices"

// GetOrAdd returns existing value or adds new one.
func GetOrAdd[T any](slc *slices.Sorted[T], v T) T {
	var index int
	if index = slc.Index(v); index == -1 {
		index = slc.Add(v)
	}

	return slc.Get(index)
}

// AddIfNotFound adds value to the slice if it's not found.
func AddIfNotFound[T any](slc *slices.Sorted[T], v T) {
	if index := slc.Index(v); index == -1 {
		slc.Add(v)
	}
}
