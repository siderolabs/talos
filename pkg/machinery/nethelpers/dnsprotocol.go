// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// DNSProtocol is a kind of DNS protocol.
type DNSProtocol byte

// DNSProtocol constants.
//
//structprotogen:gen_enum
const (
	DNSProtocolDefault     DNSProtocol = iota // Do53
	DNSProtocolDNSOverTLS                     // DoT
	DNSProtocolDNSOverHTTP                    // DoH
)
