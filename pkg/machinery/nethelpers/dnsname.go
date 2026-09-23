// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "fmt"

// ValidateDNSNameChars checks that a hostname, domain name or search domain doesn't contain
// whitespace or control characters.
//
// Such characters are never valid in DNS names.
//
// The check is intentionally permissive otherwise (e.g. underscores, uppercase or non-ASCII characters are allowed),
// as in general a hostname in UNIX is not strictly limited to the characters allowed in DNS names.
//
// The check runs over bytes of the string, so it does not validate that the string is valid UTF-8.
func ValidateDNSNameChars(name string) error {
	for i := range len(name) {
		if c := name[i]; c <= ' ' || c == 0x7f {
			return fmt.Errorf("name %q contains invalid character %q at position %d", name, c, i)
		}
	}

	return nil
}
