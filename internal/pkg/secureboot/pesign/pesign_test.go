// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pesign_test

import (
	"crypto"
	stdx509 "crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/secureboot/pesign"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
)

type certificateProvider struct {
	*x509.CertificateAuthority
}

func (c *certificateProvider) Signer() crypto.Signer {
	return c.CertificateAuthority.Key.(crypto.Signer)
}

func (c *certificateProvider) Certificate() *stdx509.Certificate {
	return c.CertificateAuthority.Crt
}

func TestSign(t *testing.T) {
	currentTime := time.Now()

	opts := []x509.Option{
		x509.RSA(true),
		x509.Bits(2048),
		x509.CommonName("test-sign"),
		x509.NotAfter(currentTime.Add(secrets.CAValidityTime)),
		x509.NotBefore(currentTime),
		x509.Organization("test-sign"),
	}

	tmpDir := t.TempDir()

	signingKey, err := x509.NewSelfSignedCertificateAuthority(opts...)
	require.NoError(t, err)

	signer, err := pesign.NewSigner(&certificateProvider{signingKey})
	require.NoError(t, err)

	require.NoError(t, signer.Sign("testdata/systemd-bootx64.efi", filepath.Join(tmpDir, "boot.efi")))

	unsigned, err := os.Stat("testdata/systemd-bootx64.efi")
	require.NoError(t, err)

	signed, err := os.Stat(filepath.Join(tmpDir, "boot.efi"))
	require.NoError(t, err)

	require.Greater(t, signed.Size(), unsigned.Size())
}
