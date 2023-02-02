// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

// Quote exported for testing.
func Quote(s string) string {
	return quote(s)
}

// Unquote exported for testing.
func Unquote(s string) string {
	return unquote(s)
}
