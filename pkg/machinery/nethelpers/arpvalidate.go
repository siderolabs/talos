// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "fmt"

//go:generate enumer -type=ARPValidate -linecomment -text

// ARPValidate is an ARP Validation mode.
type ARPValidate uint32

// ARPValidate constants.
const (
	ARPValidateNone   ARPValidate = iota // none
	ARPValidateActive                    // active
	ARPValidateBackup                    // backup
	ARPValidateAll                       // all
)

// ARPValidateByName parses ARPValidate.
func ARPValidateByName(a string) (ARPValidate, error) {
	switch a {
	case "", "none":
		return ARPValidateNone, nil
	case "active":
		return ARPValidateActive, nil
	case "backup":
		return ARPValidateBackup, nil
	case "all":
		return ARPValidateAll, nil
	default:
		return 0, fmt.Errorf("invalid arp_validate mode %v", a)
	}
}
