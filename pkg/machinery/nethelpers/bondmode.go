// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "fmt"

//go:generate enumer -type=BondMode -linecomment -text

// BondMode is a bond mode.
type BondMode uint8

// BondMode constants.
//
// See linux/if_bonding.h.
const (
	BondModeRoundrobin   BondMode = iota // balance-rr
	BondModeActiveBackup                 // active-backup
	BondModeXOR                          // balance-xor
	BondModeBroadcast                    // broadcast
	BondMode8023AD                       // 802.3ad
	BondModeTLB                          // balance-tlb
	BondModeALB                          // balance-alb
)

// BondModeByName converts string bond mode into a constant.
func BondModeByName(mode string) (bm BondMode, err error) {
	switch mode {
	case "", "balance-rr":
		return BondModeRoundrobin, nil
	case "active-backup":
		return BondModeActiveBackup, nil
	case "balance-xor":
		return BondModeXOR, nil
	case "broadcast":
		return BondModeBroadcast, nil
	case "802.3ad":
		return BondMode8023AD, nil
	case "balance-tlb":
		return BondModeTLB, nil
	case "balance-alb":
		return BondModeALB, nil
	default:
		return 0, fmt.Errorf("invalid bond type %s", mode)
	}
}
