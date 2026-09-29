// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

func TestValidateDNSNameChars(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"",
		"foo",
		"foo.example.com",
		"foo.example.com.",
		"Foo_Bar-1",
		"xn--80ak6aa92e.com",
		"пример.рф",
		"weird#name;",
	} {
		assert.NoError(t, nethelpers.ValidateDNSNameChars(name), "name %q", name)
	}

	for _, name := range []string{
		"foo bar",
		"foo\tbar",
		"poc.example\nnameserver 198.51.100.66",
		"foo\r",
		"foo\x00",
		"foo\x7f",
		"foo\x1b[31m",
	} {
		assert.Error(t, nethelpers.ValidateDNSNameChars(name), "name %q", name)
	}
}
