// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// AddressSortAlgorithm is an internal address sorting algorithm.
type AddressSortAlgorithm int

// AddressSortAlgorithm constants.
//
//structprotogen:gen_enum
const (
	AddressSortAlgorithmV1 AddressSortAlgorithm = iota // v1
	AddressSortAlgorithmV2                             // v2
)
