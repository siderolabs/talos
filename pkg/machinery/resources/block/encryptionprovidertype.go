// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

// EncryptionProviderType describes encryption provider type.
type EncryptionProviderType int

// Encryption provider types.
//
//structprotogen:gen_enum
const (
	EncryptionProviderNone  EncryptionProviderType = iota // none
	EncryptionProviderLUKS2                               // luks2
)
