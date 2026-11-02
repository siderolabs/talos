// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package basic_test

import (
	"context"
	"crypto/tls"
	stdx509 "crypto/x509"
	"encoding/asn1"
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/grpc/middleware/auth/basic"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func newServerCert(t *testing.T, ca *x509.CertificateAuthority, opts ...x509.Option) tls.Certificate {
	t.Helper()

	keyPair, err := x509.NewKeyPair(ca,
		append([]x509.Option{
			x509.DNSNames([]string{"trustd.example.com"}),
			x509.KeyUsage(stdx509.KeyUsageDigitalSignature),
			x509.ExtKeyUsage([]stdx509.ExtKeyUsage{stdx509.ExtKeyUsageServerAuth}),
		}, opts...)...,
	)
	require.NoError(t, err)

	return *keyPair.Certificate
}

//nolint:gocyclo
func TestTLSConfigServerExtKeyUsage(t *testing.T) {
	t.Parallel()

	ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true))
	require.NoError(t, err)

	otherCA, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true))
	require.NoError(t, err)

	trustdEKU := x509.UnknownExtKeyUsage([]asn1.ObjectIdentifier{constants.TrustdServerExtKeyUsage})

	plainCert := newServerCert(t, ca)
	trustdCert := newServerCert(t, ca, trustdEKU)
	otherEKUCert := newServerCert(t, ca, x509.UnknownExtKeyUsage([]asn1.ObjectIdentifier{{1, 3, 6, 1, 4, 1, 32473, 1, 2}}))
	otherCATrustdCert := newServerCert(t, otherCA, trustdEKU)

	acceptedCAs := []*x509.PEMEncodedCertificate{{Crt: ca.CrtPEM}}

	for _, test := range []struct {
		name       string
		serverCert tls.Certificate
		opts       []basic.ConnectionOption

		expectedError string
	}{
		{
			name:       "legacy: plain certificate",
			serverCert: plainCert,
		},
		{
			name:       "legacy: trustd certificate",
			serverCert: trustdCert,
		},
		{
			name:          "legacy: wrong CA",
			serverCert:    otherCATrustdCert,
			expectedError: "certificate signed by unknown authority",
		},
		{
			name:          "strict: plain certificate",
			serverCert:    plainCert,
			opts:          []basic.ConnectionOption{basic.WithRequiredServerExtKeyUsage(constants.TrustdServerExtKeyUsage)},
			expectedError: "server certificate doesn't have the required extended key usage",
		},
		{
			name:          "strict: other custom EKU",
			serverCert:    otherEKUCert,
			opts:          []basic.ConnectionOption{basic.WithRequiredServerExtKeyUsage(constants.TrustdServerExtKeyUsage)},
			expectedError: "server certificate doesn't have the required extended key usage",
		},
		{
			name:       "strict: trustd certificate",
			serverCert: trustdCert,
			opts:       []basic.ConnectionOption{basic.WithRequiredServerExtKeyUsage(constants.TrustdServerExtKeyUsage)},
		},
		{
			name:          "strict: wrong CA",
			serverCert:    otherCATrustdCert,
			opts:          []basic.ConnectionOption{basic.WithRequiredServerExtKeyUsage(constants.TrustdServerExtKeyUsage)},
			expectedError: "certificate signed by unknown authority",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			// use a real TCP connection: with net.Pipe (unbuffered), the handshake might deadlock
			// when the client rejects the server certificate and sends an alert while the server is still writing
			listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
				Certificates: []tls.Certificate{test.serverCert},
			})
			require.NoError(t, err)

			t.Cleanup(func() {
				listener.Close() //nolint:errcheck
			})

			go func() {
				conn, acceptErr := listener.Accept()
				if acceptErr != nil {
					return
				}

				defer conn.Close() //nolint:errcheck

				conn.(*tls.Conn).HandshakeContext(ctx) //nolint:errcheck,forcetypeassert
			}()

			clientConfig := basic.NewTLSConfig(acceptedCAs, test.opts...)
			clientConfig.ServerName = "trustd.example.com"

			dialer := tls.Dialer{Config: clientConfig}

			conn, err := dialer.DialContext(ctx, "tcp", listener.Addr().String())
			if err == nil {
				conn.Close() //nolint:errcheck
			}

			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.expectedError)
			}
		})
	}
}
