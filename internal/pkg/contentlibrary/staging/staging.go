// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package staging describes how an in-flight upload to a content library is named on disk.
//
// Staged names are dot-prefixed, which keeps them out of the content library API entirely: they are
// invisible to List and unaddressable by Delete, so an interrupted upload can never be cleared over
// the API. The suffix is what lets whatever is left behind be swept instead.
package staging

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

const (
	// Prefix starts the name an upload is staged under.
	Prefix = "."
	// Suffix ends it.
	Suffix = ".upload"
)

// NameOverhead is what Name adds to a name: the prefix, a separator, a hex discriminator and the
// suffix.
const NameOverhead = len(Prefix) + 1 + 16 + len(Suffix)

// Name returns the name an upload of name is staged under.
//
// Unique per upload, so that a staged file left behind by an interrupted one can never block a
// later upload of the same name.
func Name(name string) string {
	return fmt.Sprintf("%s%s.%016x%s", Prefix, name, rand.Uint64(), Suffix)
}

// IsName reports whether name is an upload staged by Name.
func IsName(name string) bool {
	return strings.HasPrefix(name, Prefix) && strings.HasSuffix(name, Suffix)
}
