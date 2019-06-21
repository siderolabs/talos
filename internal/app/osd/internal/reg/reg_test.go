/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func TestToCIDR(t *testing.T) {
	assert.Equal(t, toCIDR(unix.AF_INET, net.ParseIP("192.168.254.0"), 24), "192.168.254.0/24")
	assert.Equal(t, toCIDR(unix.AF_INET6, net.ParseIP("2001:db8::"), 16), "2001:db8::/16")
}
