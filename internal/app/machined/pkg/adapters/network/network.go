// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network implements adapters wrapping resources/network to provide additional functionality.
package network

// MSS calculation constants.
const (
	IPv4HeaderLen = 20 // IPv4 fixed header length
	IPv6HeaderLen = 40 // IPv6 fixed header length
	TCPHeaderLen  = 20 // fixed TCP header length, without options
	TCPOptionsLen = 12 // assuming typical options like SACK, timestamps, etc. used by default in Linux
)
