// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// ClientIdentifier is a DHCP client identifier.
type ClientIdentifier int

// ClientIdentifier constants.
//
//structprotogen:gen_enum
const (
	ClientIdentifierNone ClientIdentifier = iota // none
	ClientIdentifierMAC                          // mac
	ClientIdentifierDUID                         // duid
)
