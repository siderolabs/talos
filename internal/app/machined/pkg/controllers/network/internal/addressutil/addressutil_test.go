// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package addressutil_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/addressutil"
)

func TestDeduplicateIPPrefixes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		in   []netip.Prefix

		out []netip.Prefix
	}{
		{
			name: "empty",
		},
		{
			name: "single",
			in:   []netip.Prefix{netip.MustParsePrefix("1.2.3.4/32"), netip.MustParsePrefix("1.2.3.4/32")},

			out: []netip.Prefix{netip.MustParsePrefix("1.2.3.4/32")},
		},
		{
			name: "many",
			in:   []netip.Prefix{netip.MustParsePrefix("1.2.3.4/32"), netip.MustParsePrefix("1.2.3.4/24"), netip.MustParsePrefix("2000::aebc/64"), netip.MustParsePrefix("2000::aebc/64")},

			out: []netip.Prefix{netip.MustParsePrefix("1.2.3.4/32"), netip.MustParsePrefix("1.2.3.4/24"), netip.MustParsePrefix("2000::aebc/64")},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := addressutil.DeduplicateIPPrefixes(test.in)

			assert.Equal(t, test.out, got)
		})
	}
}

// TestFilterIPs tests the FilterIPs function.
func TestFilterIPs(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		in      []netip.Prefix
		include []netip.Prefix
		exclude []netip.Prefix

		out []netip.Prefix
	}{
		{
			name: "empty filters",

			in: []netip.Prefix{netip.MustParsePrefix("1.2.3.4/32"), netip.MustParsePrefix("2000::aebc/64")},

			out: []netip.Prefix{netip.MustParsePrefix("1.2.3.4/32"), netip.MustParsePrefix("2000::aebc/64")},
		},
		{
			name: "v4 only",

			in:      []netip.Prefix{netip.MustParsePrefix("1.2.3.4/32"), netip.MustParsePrefix("2000::aebc/64")},
			include: []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")},

			out: []netip.Prefix{netip.MustParsePrefix("1.2.3.4/32")},
		},
		{
			name: "v6 only",

			in:      []netip.Prefix{netip.MustParsePrefix("1.2.3.4/32"), netip.MustParsePrefix("2000::aebc/64")},
			exclude: []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")},

			out: []netip.Prefix{netip.MustParsePrefix("2000::aebc/64")},
		},
		{
			name: "include and exclude",

			in:      []netip.Prefix{netip.MustParsePrefix("1.2.3.4/32"), netip.MustParsePrefix("3.4.5.6/24"), netip.MustParsePrefix("2000::aebc/64")},
			include: []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")},
			exclude: []netip.Prefix{netip.MustParsePrefix("3.0.0.0/8")},

			out: []netip.Prefix{netip.MustParsePrefix("1.2.3.4/32")},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := addressutil.FilterIPs(test.in, test.include, test.exclude)

			assert.Equal(t, test.out, got)
		})
	}
}
