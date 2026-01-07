// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basic_test

import (
	"testing"

	"github.com/siderolabs/talos/pkg/grpc/middleware/auth/basic"
)

func TestParseAuthority(t *testing.T) {
	cases := []struct {
		host string
		want string
	}{
		{"127.0.0.1", ""},
		{"127.0.0.1:443", ""},
		{"[::1]", ""},
		{"[::1]:443", ""},
		{"example.com", "example.com"},
		{"example.com:443", "example.com"},
		{"[example.com]:443", "example.com"},
		{"", ""},
	}
	for _, c := range cases {
		got := basic.ParseAuthority(c.host)
		if got != c.want {
			t.Fatalf("ParseAuthority(%q) = %q, want %q", c.host, got, c.want)
		}
	}
}
