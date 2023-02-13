// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"strings"
)

// quote according to (incomplete) GRUB quoting rules.
//
// See https://www.gnu.org/software/grub/manual/grub/html_node/Shell_002dlike-scripting.html
func quote(s string) string {
	for _, c := range `\{}&$|;<>"` {
		s = strings.ReplaceAll(s, string(c), `\`+string(c))
	}

	return s
}

// unquote according to (incomplete) GRUB quoting rules.
func unquote(s string) string {
	for _, c := range `{}&$|;<>\"` {
		s = strings.ReplaceAll(s, `\`+string(c), string(c))
	}

	return s
}
