// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package pemutils provides helpers for working with PEM-encoded data.
package pemutils

import (
	"github.com/siderolabs/crypto/x509"
)

// Join concatenates PEM-encoded blocks into a single bundle.
//
// Each non-empty block is terminated with a newline if it doesn't have one,
// so that the END line of one block never runs into the BEGIN line of the next one.
// Blocks which are already newline-terminated are concatenated verbatim.
func Join(blocks ...[]byte) []byte {
	size := 0

	for _, block := range blocks {
		size += len(block) + 1
	}

	result := make([]byte, 0, size)

	for _, block := range blocks {
		if len(block) == 0 {
			continue
		}

		result = append(result, block...)

		if block[len(block)-1] != '\n' {
			result = append(result, '\n')
		}
	}

	return result
}

// JoinCertificates concatenates PEM-encoded certificates into a single bundle (e.g. a list of accepted CAs).
//
// See Join for details.
func JoinCertificates(certs []*x509.PEMEncodedCertificate) []byte {
	blocks := make([][]byte, 0, len(certs))

	for _, cert := range certs {
		if cert == nil {
			continue
		}

		blocks = append(blocks, cert.Crt)
	}

	return Join(blocks...)
}

// JoinKeys concatenates PEM-encoded keys into a single bundle.
//
// See Join for details.
func JoinKeys(keys []*x509.PEMEncodedKey) []byte {
	blocks := make([][]byte, 0, len(keys))

	for _, key := range keys {
		if key == nil {
			continue
		}

		blocks = append(blocks, key.Key)
	}

	return Join(blocks...)
}
