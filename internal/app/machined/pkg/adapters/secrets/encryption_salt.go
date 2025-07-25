// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"crypto/rand"
	"io"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// EncryptionSalt adapter provides encryption salt generation.
//
//nolint:revive,golint
func EncryptionSalt(r *secrets.EncryptionSaltSpec) encryptionSalt {
	return encryptionSalt{
		EncryptionSaltSpec: r,
	}
}

type encryptionSalt struct {
	*secrets.EncryptionSaltSpec
}

// Generate new encryption salt.
func (a encryptionSalt) Generate() error {
	buf := make([]byte, constants.DiskEncryptionSaltSize)

	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return err
	}

	a.EncryptionSaltSpec.DiskSalt = buf

	return nil
}
