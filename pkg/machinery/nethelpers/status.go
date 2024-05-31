// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// Status is a network status.
//
// Please see resources/network/status.go.
type Status int

// Status constants.
const (
	StatusAddresses    Status = 1 // addresses
	StatusConnectivity Status = 2 // connectivity
	StatusHostname     Status = 3 // hostname
	StatusEtcFiles     Status = 4 // etcfiles
)
