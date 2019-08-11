/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package net

import (
	"testing"

	"gotest.tools/assert"
)

func TestEmpty(t *testing.T) {
	// added for accurate coverage estimation
	//
	// please remove it once any unit-test is added
	// for this package
}

func TestFormatAddress(t *testing.T) {
	assert.Equal(t, FormatAddress("2001:db8::1"), "[2001:db8::1]")
	assert.Equal(t, FormatAddress("[2001:db8::1]"), "[2001:db8::1]")
	assert.Equal(t, FormatAddress("192.168.1.1"), "192.168.1.1")
	assert.Equal(t, FormatAddress("alpha.beta.gamma.com"), "alpha.beta.gamma.com")
}
