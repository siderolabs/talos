// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !linux

// Package tpm provides TPM 2.0 support.
package tpm

import (
	"errors"

	"github.com/google/go-tpm/tpm2/transport"
)

// Open a TPM device.
func Open() (transport.TPMCloser, error) {
	return nil, errors.New("TPM device is not available")
}
