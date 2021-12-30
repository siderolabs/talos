// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "fmt"

//go:generate enumer -type=BondXmitHashPolicy -linecomment -text

// BondXmitHashPolicy is a bond hash policy.
type BondXmitHashPolicy uint8

// Bond hash policy constants.
const (
	BondXmitPolicyLayer2  BondXmitHashPolicy = iota // layer2
	BondXmitPolicyLayer34                           // layer3+4
	BondXmitPolicyLayer23                           // layer2+3
	BondXmitPolicyEncap23                           // encap2+3
	BondXmitPolicyEncap34                           // encap3+4
)

// BondXmitHashPolicyByName parses bond hash policy.
func BondXmitHashPolicyByName(policy string) (BondXmitHashPolicy, error) {
	switch policy {
	case "", "layer2":
		return BondXmitPolicyLayer2, nil
	case "layer3+4":
		return BondXmitPolicyLayer34, nil
	case "layer2+3":
		return BondXmitPolicyLayer23, nil
	case "encap2+3":
		return BondXmitPolicyEncap23, nil
	case "encap3+4":
		return BondXmitPolicyEncap34, nil
	default:
		return 0, fmt.Errorf("invalid xmit hash policy %v", policy)
	}
}
