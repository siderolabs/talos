// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package addressutil

import (
	"cmp"
	"fmt"
	"net/netip"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// CompareByAlgorithm returns a comparison function based on the given algorithm.
func CompareByAlgorithm(algorithm nethelpers.AddressSortAlgorithm) func(a, b netip.Prefix) int {
	switch algorithm {
	case nethelpers.AddressSortAlgorithmV1:
		return ComparePrefixesLegacy
	case nethelpers.AddressSortAlgorithmV2:
		return ComparePrefixNew
	}

	panic(fmt.Sprintf("unknown address sort algorithm: %s", algorithm))
}

// ComparePrefixesLegacy is the old way to sort prefixes.
//
// It only compares addresses and does not take prefix length into account.
func ComparePrefixesLegacy(a, b netip.Prefix) int {
	if c := a.Addr().Compare(b.Addr()); c != 0 {
		return c
	}

	// note: this was missing in the previous implementation, but this makes sorting stable
	return cmp.Compare(a.Bits(), b.Bits())
}

func family(a netip.Prefix) int {
	if a.Addr().Is4() {
		return 4
	}

	return 6
}

// ComparePrefixNew compares two prefixes by address family, address, and prefix length.
//
// It prefers more specific prefixes.
func ComparePrefixNew(a, b netip.Prefix) int {
	// (1): first, compare address families
	if c := cmp.Compare(family(a), family(b)); c != 0 {
		return c
	}

	// (2): if addresses are equal, Contains will report that one prefix contains the other, so compare prefix lengths
	if a.Addr() == b.Addr() {
		return -cmp.Compare(a.Bits(), b.Bits())
	}

	// (3): if one prefix contains another, the more specific one should come first
	// but if both prefixes contain each other, proceed to compare addresses
	aContainsB := a.Contains(b.Addr())
	bContainsA := b.Contains(a.Addr())

	switch {
	case aContainsB && !bContainsA:
		return 1
	case !aContainsB && bContainsA:
		return -1
	}

	// (4): compare addresses, they are not equal at this point (see (2))
	return a.Addr().Compare(b.Addr())
}

// CompareAddressStatuses compares two address statuses with the prefix comparison func.
//
// The comparison of AddressStatuses sorts by link name and then by address.
func CompareAddressStatuses(comparePrefixes func(a, b netip.Prefix) int) func(a, b *network.AddressStatus) int {
	return func(a, b *network.AddressStatus) int {
		if c := cmp.Compare(a.TypedSpec().LinkName, b.TypedSpec().LinkName); c != 0 {
			return c
		}

		return comparePrefixes(a.TypedSpec().Address, b.TypedSpec().Address)
	}
}
