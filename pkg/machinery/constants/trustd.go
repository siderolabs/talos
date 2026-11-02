// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package constants

import "encoding/asn1"

// TrustdServerExtKeyUsage is the custom extended key usage OID set on trustd server certificates.
//
// TODO: replace the placeholder 1.3.6.1.4.1.32473 (RFC 5612 example PEN) with the Sidero Labs PEN.
var TrustdServerExtKeyUsage = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 32473, 1, 1}
