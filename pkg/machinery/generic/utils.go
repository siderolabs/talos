// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generic

// IsZero tests if generic value T is zero.
func IsZero[T comparable](t T) bool {
	var zero T

	return t == zero
}
