// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pcr_test

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/google/go-tpm/tpm2"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/measure/internal/pcr"
	tpm2internal "github.com/siderolabs/talos/internal/pkg/secureboot/tpm2"
)

type keyWrapper struct {
	*rsa.PrivateKey
}

func (k keyWrapper) PublicRSAKey() *rsa.PublicKey {
	return &k.PrivateKey.PublicKey
}

func TestCalculateBankData(t *testing.T) {
	t.Parallel()

	pemKey, err := os.ReadFile("../../testdata/pcr-signing-key.pem")
	require.NoError(t, err)

	block, _ := pem.Decode(pemKey)
	require.NotNil(t, block)

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)

	bankData, err := pcr.CalculateBankData(15, tpm2.TPMAlgSHA256,
		map[string]string{
			".initrd": "testdata/a",
			".linux":  "testdata/b",
			".dtb":    "testdata/c",
		},
		keyWrapper{key})
	require.NoError(t, err)

	require.Equal(t,
		[]tpm2internal.BankData{
			{
				PCRs: []int{15},
				PKFP: "58f58f625bd8a8b6681e4b40688cf99b26419b6b2c5f6e14a2c7c67a3b0b1620",
				Pol:  "a1c9d366451c82969238eb82a5282f84b6a3d499e540430ecf792083155225dd",
				Sig:  "bO4F/T6bio7jLJFpi4GsJHjZDj+H5Pq4stjKA5WhkzBNCmE1gQECeOALUfNJ1RW/HKhwSp7KhGFqqnjyg/eXR3c0pVuYUKuAjZz9NMXS4dAQlSLxtNWMmlX3XDst/UKfxB6Z+m2KluJpF3KeAw03tP9lru6nfzaickOs1UL83IO5QgLkCHpUcSloZcya0xS9ETCNBd5gm8K+c9gc7+CmpFLTo1uTxbBK0Mea+3fn7GAZROHPMBLosvTM5D9vplsWIXXAXSaHr/sj5bxOIR+orCQZOdYY+/8ra4oFVXzHc9kkPP3A6mWzoADKryWWIVKPx/DGLi0ExT2fpCNdUoMOacvD+dqDqjVBhcOwoAZkNqve/W/poqaLlKyFTqlmGmv+08WavtShYmCURa4Mn3UFf49BTVkktxoQ+jTMroyit1uK/ppMSjaPwQp2Dd1pRCY4hcFfLwqryy1zRMT/XmZ2e91MYe40Pr9Tom2ZH0YAigDosBPuP6RHt7IypFIgary3louW1dqNLXW8p38Y91nYDKBWI9x0tVn5ufqtk5wkHnExTjUYkTWU98+p5J7urDIhLuX1mSi57Ekq02f+lVLMs85SHfmMfzZl7l7Xi4npYbW+5xHKiAxLnaXVJCHdW0xiAD0VTLer5Oe5nf7FrjSzS39rXoryKfcHFOIxRT1XQOA=", //nolint:lll
			},
		}, bankData)
}
