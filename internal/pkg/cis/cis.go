// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cis

import (
	"crypto/rand"
	"encoding/base64"
)

// CreateEncryptionToken generates an encryption token to be used for secrets.
func CreateEncryptionToken() (string, error) {
	encryptionKey := make([]byte, 32)
	if _, err := rand.Read(encryptionKey); err != nil {
		return "", err
	}

	str := base64.StdEncoding.EncodeToString(encryptionKey)

	return str, nil
}
