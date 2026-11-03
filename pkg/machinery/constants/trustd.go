// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package constants

import "encoding/asn1"

// SideroLabsPEN is the IANA Private Enterprise Number assigned to Sidero Labs.
const SideroLabsPEN = 67497

// TrustdServerExtKeyUsage is the custom extended key usage OID set on trustd server certificates.
//
// OID layout under the Sidero Labs PEN (1.3.6.1.4.1.67497):
//
//	.1       Talos
//	.1.1     Talos extended key usages
//	.1.1.1   trustd server
var TrustdServerExtKeyUsage = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, SideroLabsPEN, 1, 1, 1}
