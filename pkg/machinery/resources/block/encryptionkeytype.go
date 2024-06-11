// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

// EncryptionKeyType describes encryption key type.
type EncryptionKeyType int

// Encryption key types.
//
//structprotogen:gen_enum
const (
	EncryptionKeyStatic EncryptionKeyType = iota // static
	EncryptionKeyNodeID                          // nodeID
	EncryptionKeyKMS                             // kms
	EncryptionKeyTPM                             // tpm
)
