// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package addressutil_test

import (
	"math/rand/v2"
	"net/netip"
	"slices"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/addressutil"
)

func toNetip(prefixes ...string) []netip.Prefix {
	return xslices.Map(prefixes, netip.MustParsePrefix)
}

func toString(prefixes []netip.Prefix) []string {
	return xslices.Map(prefixes, netip.Prefix.String)
}

func TestCompare(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		in []netip.Prefix

		outLegacy []netip.Prefix
		outNew    []netip.Prefix
	}{
		{
			name: "ipv4",

			in: toNetip("10.3.4.1/24", "10.3.4.5/24", "10.3.4.5/32", "1.2.3.4/26", "192.168.35.11/24", "192.168.36.10/24"),

			outLegacy: toNetip("1.2.3.4/26", "10.3.4.1/24", "10.3.4.5/24", "10.3.4.5/32", "192.168.35.11/24", "192.168.36.10/24"),
			outNew:    toNetip("1.2.3.4/26", "10.3.4.5/32", "10.3.4.1/24", "10.3.4.5/24", "192.168.35.11/24", "192.168.36.10/24"),
		},
		{
			name: "ipv6",

			in: toNetip("2001:db8::1/64", "2001:db8::1/128", "2001:db8::2/64", "2001:db8::2/128", "2001:db8::3/64", "2001:db8::3/128"),

			outLegacy: toNetip("2001:db8::1/64", "2001:db8::1/128", "2001:db8::2/64", "2001:db8::2/128", "2001:db8::3/64", "2001:db8::3/128"),
			outNew:    toNetip("2001:db8::1/128", "2001:db8::2/128", "2001:db8::3/128", "2001:db8::1/64", "2001:db8::2/64", "2001:db8::3/64"),
		},
		{
			name: "mixed",

			in: toNetip("fd01:cafe::5054:ff:fe1f:c7bd/64", "fd01:cafe::f14c:9fa1:8496:557f/128", "192.168.3.4/24", "10.5.0.0/16"),

			outLegacy: toNetip("10.5.0.0/16", "192.168.3.4/24", "fd01:cafe::5054:ff:fe1f:c7bd/64", "fd01:cafe::f14c:9fa1:8496:557f/128"),
			outNew:    toNetip("10.5.0.0/16", "192.168.3.4/24", "fd01:cafe::f14c:9fa1:8496:557f/128", "fd01:cafe::5054:ff:fe1f:c7bd/64"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// add more randomness to ensure the sorting is stable
			in := slices.Clone(test.in)
			rand.Shuffle(len(in), func(i, j int) { in[i], in[j] = in[j], in[i] })

			legacy := slices.Clone(in)
			slices.SortFunc(legacy, addressutil.ComparePrefixesLegacy)

			assert.Equal(t, test.outLegacy, legacy, "expected %q but got %q", toString(test.outLegacy), toString(legacy))

			newer := slices.Clone(in)
			slices.SortFunc(newer, addressutil.ComparePrefixNew)

			assert.Equal(t, test.outNew, newer, "expected %q but got %q", toString(test.outNew), toString(newer))
		})
	}
}
