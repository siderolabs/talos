// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tpm2 provides TPM2.0 related functionality helpers.
package tpm2_test

import (
	"testing"

	"github.com/google/go-tpm/tpm2"
	"github.com/stretchr/testify/require"

	tpm2internal "github.com/siderolabs/talos/internal/pkg/secureboot/tpm2"
)

func TestCalculatePolicy(t *testing.T) {
	t.Parallel()

	policy, err := tpm2internal.CalculatePolicy([]byte{1, 3, 5}, tpm2.TPMLPCRSelection{
		PCRSelections: []tpm2.TPMSPCRSelection{
			{
				Hash:      tpm2.TPMAlgSHA256,
				PCRSelect: []byte{10, 11, 12},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t,
		[]byte{0x84, 0xd6, 0x51, 0x47, 0xb0, 0x53, 0x94, 0xd0, 0xfa, 0xc4, 0x5e, 0x36, 0x0, 0x20, 0x3e, 0x3a, 0x11, 0x1, 0x27, 0xfb, 0xe2, 0x6f, 0xc1, 0xe3, 0x3, 0x3, 0x10, 0x21, 0x33, 0xf9, 0x15, 0xe3},
		policy,
	)
}

func TestCalculateSealingPolicyDigest(t *testing.T) {
	t.Parallel()

	calculated, err := tpm2internal.CalculateSealingPolicyDigest([]byte{1, 3, 5}, tpm2.TPMLPCRSelection{
		PCRSelections: []tpm2.TPMSPCRSelection{
			{
				Hash:      tpm2.TPMAlgSHA256,
				PCRSelect: []byte{10, 11, 12},
			},
		},
	}, "testdata/pcr-signing-crt.pem")
	require.NoError(t, err)
	require.Equal(t,
		[]byte{0xa0, 0xf4, 0x29, 0x7a, 0x6c, 0x1a, 0xc8, 0xcf, 0x88, 0x48, 0x8b, 0xcf, 0x63, 0x20, 0xdc, 0x2e, 0x75, 0xc8, 0x2, 0xa1, 0xb4, 0x62, 0x5a, 0xdc, 0x9a, 0xfb, 0x2a, 0x1a, 0x8a, 0xd2, 0xf0, 0x53},
		calculated,
	)
}
