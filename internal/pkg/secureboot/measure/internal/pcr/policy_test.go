// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pcr_test

import (
	"testing"

	"github.com/google/go-tpm/tpm2"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/secureboot/measure/internal/pcr"
)

func TestCalculatePolicy(t *testing.T) {
	t.Parallel()

	policy, err := pcr.CalculatePolicy([]byte{1, 3, 5}, tpm2.TPMLPCRSelection{
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
