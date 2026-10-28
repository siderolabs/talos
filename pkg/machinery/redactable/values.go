// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package redactable

import (
	"slices"
	"strings"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// RedactKeyValue replaces the value in a "key=value" string with [constants.Redacted], keeping the
// key, and reports whether it changed anything.
//
// This is how a command line flag or an environment variable is redacted: which flags are set and
// which variables are defined is what makes the resource worth reading at all, while the values are
// copied from the machine configuration verbatim and so have to be assumed to hold secrets.
//
// A string with no "=" in it, or with nothing after it, is left alone: it names no value to take out.
func RedactKeyValue(s string) (string, bool) {
	key, value, found := strings.Cut(s, "=")
	if !found || value == "" {
		return s, false
	}

	return key + "=" + constants.Redacted, true
}

// HasKeyValues reports whether the list carries a "key=value" with a value in it.
func HasKeyValues(values []string) bool {
	return slices.ContainsFunc(values, func(value string) bool {
		_, redacted := RedactKeyValue(value)

		return redacted
	})
}

// RedactKeyValues returns a copy of the list with the value of every "key=value" replaced, or the
// list itself when there is nothing to take out.
//
// A plain entry with no "=" in it -- the binary to run, a bare flag -- names no value and is left as
// it is. A flag which carries its value in the next entry ("--flag", "value") keeps it for the same
// reason, which is why Talos renders every flag it passes with the "=" in it, see the args builder.
func RedactKeyValues(values []string) []string {
	if !HasKeyValues(values) {
		return values
	}

	return xslices.Map(values, func(value string) string {
		redacted, _ := RedactKeyValue(value)

		return redacted
	})
}
