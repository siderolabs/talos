// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package fipsmode provides a way to check if the system is running in FIPS mode.
package fipsmode

import (
	"crypto/fips140"
	"crypto/sha1"
	"sync"
)

// Enabled checks if the system is running in FIPS mode.
func Enabled() bool {
	return fips140.Enabled()
}

// Strict checks if the strict FIPS mode is enabled.
//
// Go doesn't provide a simple way to check for strict FIPS mode, so we
// use a side-effect of SHA-1 to fail.
var Strict = sync.OnceValue(func() bool {
	if !Enabled() {
		return false
	}

	_, err := sha1.New().Write(nil)

	strict := err != nil

	return strict
})
