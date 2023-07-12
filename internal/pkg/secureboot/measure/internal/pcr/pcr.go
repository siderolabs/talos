// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package pcr contains code that handles PCR operations.
package pcr

import "fmt"

// CreateSelector converts PCR  numbers into a bitmask.
func CreateSelector(pcrs []int) ([]byte, error) {
	const sizeOfPCRSelect = 3

	mask := make([]byte, sizeOfPCRSelect)

	for _, n := range pcrs {
		if n >= 8*sizeOfPCRSelect {
			return nil, fmt.Errorf("PCR index %d is out of range (exceeds maximum value %d)", n, 8*sizeOfPCRSelect-1)
		}

		mask[n>>3] |= 1 << (n & 0x7)
	}

	return mask, nil
}
