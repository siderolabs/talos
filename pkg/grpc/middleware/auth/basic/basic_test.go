// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basic_test

import (
	"testing"

	"github.com/siderolabs/talos/pkg/grpc/middleware/auth/basic"
	"github.com/stretchr/testify/assert"
)

func TestParseAuthority(t *testing.T) {
	for _, tc := range []struct {
		host string
		want string
	}{
		{"", ""},
		{"::1", ""},
		{"[::1]", ""},
		{"[::1]:443", ""},
		{"127.0.0.1", ""},
		{"127.0.0.1:443", ""},
		{"[example.com]", ""},
		{"example.com", "example.com"},
		{"example.com:443", "example.com"},
		{"[example.com]:443", "example.com"},
	} {
		assert.Equalf(t, tc.want, basic.ParseAuthority(tc.host), "ParseAuthority(%q)", tc.host)
	}
}
