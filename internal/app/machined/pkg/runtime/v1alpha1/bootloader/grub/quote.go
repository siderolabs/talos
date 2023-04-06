// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"strings"
)

// Quote according to (incomplete) GRUB quoting rules.
//
// See https://www.gnu.org/software/grub/manual/grub/html_node/Shell_002dlike-scripting.html
func Quote(s string) string {
	for _, c := range `\{}&$|;<>"` {
		s = strings.ReplaceAll(s, string(c), `\`+string(c))
	}

	return s
}

// Unquote according to (incomplete) GRUB quoting rules.
func Unquote(s string) string {
	for _, c := range `{}&$|;<>\"` {
		s = strings.ReplaceAll(s, `\`+string(c), string(c))
	}

	return s
}
