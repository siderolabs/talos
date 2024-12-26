// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

// Package tpm provides TPM 2.0 support.
package tpm

import (
	"errors"
	"os"

	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/linuxtpm"
)

// Open a TPM device.
//
// Tries first /dev/tpmrm0 and then /dev/tpm0.
func Open() (transport.TPMCloser, error) {
	tpm, err := linuxtpm.Open("/dev/tpmrm0")

	if errors.Is(err, os.ErrNotExist) {
		tpm, err = linuxtpm.Open("/dev/tpm0")
	}

	return tpm, err
}
