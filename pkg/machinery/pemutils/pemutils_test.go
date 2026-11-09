// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pemutils_test

import (
	"bytes"
	stdx509 "crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/pemutils"
)

func TestJoin(t *testing.T) {
	t.Parallel()

	assert.Empty(t, pemutils.Join())
	assert.Empty(t, pemutils.Join(nil, []byte{}))
	assert.Equal(t, "a\nb\n", string(pemutils.Join([]byte("a\n"), []byte("b\n"))))
	assert.Equal(t, "a\nb\n", string(pemutils.Join([]byte("a"), nil, []byte("b"))))
	assert.Equal(t, "a\n\nb\n", string(pemutils.Join([]byte("a\n\n"), []byte("b"))))
}

func TestJoinCertificates(t *testing.T) {
	t.Parallel()

	newCA := func(org string) []byte {
		ca, err := x509.NewSelfSignedCertificateAuthority(x509.Organization(org), x509.NotAfter(time.Now().Add(time.Hour)))
		require.NoError(t, err)

		return ca.CrtPEM
	}

	ca1, ca2, ca3 := newCA("one"), newCA("two"), newCA("three")

	countCerts := func(bundle []byte) int {
		count := 0

		for {
			var block *pem.Block

			block, bundle = pem.Decode(bundle)
			if block == nil {
				return count
			}

			_, err := stdx509.ParseCertificate(block.Bytes)
			require.NoError(t, err)

			count++
		}
	}

	// well-formed certificates are concatenated verbatim
	assert.Equal(
		t,
		string(ca1)+string(ca2),
		string(pemutils.JoinCertificates([]*x509.PEMEncodedCertificate{{Crt: ca1}, {Crt: ca2}})),
	)

	// certificates without a trailing newline (e.g. YAML `|-` block) still produce a valid bundle
	bundle := pemutils.JoinCertificates([]*x509.PEMEncodedCertificate{
		{Crt: bytes.TrimSpace(ca1)},
		nil,
		{Crt: bytes.TrimSpace(ca2)},
		{Crt: ca3},
	})

	assert.Equal(t, 3, countCerts(bundle))

	pool := stdx509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(bundle))

	assert.Empty(t, pemutils.JoinCertificates(nil))
}

func TestJoinKeys(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "k1\nk2\n", string(pemutils.JoinKeys([]*x509.PEMEncodedKey{{Key: []byte("k1")}, nil, {Key: []byte("k2\n")}})))
}
