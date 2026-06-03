// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ntp

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"

	"github.com/beevik/nts"

	"github.com/siderolabs/talos/pkg/httpdefaults"
)

// DefaultNTSNewSession creates a real NTS session using beevik/nts.
// This is the default NTSNewSessionFunc used in production.
//
// When skipCertTimeCheck is true, the TLS certificate chain and hostname are
// still fully validated, but the certificate validity period (notBefore /
// notAfter) is ignored. This is used to bootstrap NTS when the system clock is
// not yet set (e.g. at boot without an RTC), where an otherwise valid server
// certificate would be rejected as "expired or not yet valid".
func DefaultNTSNewSession(address string, skipCertTimeCheck bool) (NTSSession, error) {
	tlsConfig := &tls.Config{
		RootCAs: httpdefaults.RootCAs(),
	}

	if skipCertTimeCheck {
		serverName := address
		if host, _, err := net.SplitHostPort(address); err == nil {
			serverName = host
		}

		// Disable the default verification (which enforces the validity period
		// against the current system time) and run our own verification that
		// ignores the certificate time constraints only.
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyConnection = verifyCertIgnoringTime(tlsConfig.RootCAs, serverName)
	}

	return nts.NewSessionWithOptions(
		address,
		&nts.SessionOptions{
			TLSConfig: tlsConfig,
		},
	)
}

// verifyCertIgnoringTime builds a tls.Config.VerifyConnection callback which
// validates the peer certificate chain and hostname against the provided roots
// but ignores the certificate validity period.
//
// The verification time is pinned to the leaf certificate's own NotBefore, so a
// wildly inaccurate (or unset) system clock does not cause a spurious validity
// failure. The certificate chain and hostname (SAN) are still fully verified,
// so an unknown-authority or hostname-mismatch certificate is still rejected.
func verifyCertIgnoringTime(roots *x509.CertPool, serverName string) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 {
			return errors.New("nts: server presented no certificates")
		}

		leaf := cs.PeerCertificates[0]

		intermediates := x509.NewCertPool()
		for _, cert := range cs.PeerCertificates[1:] {
			intermediates.AddCert(cert)
		}

		_, err := leaf.Verify(x509.VerifyOptions{
			Roots:         roots,
			Intermediates: intermediates,
			DNSName:       serverName,
			CurrentTime:   leaf.NotBefore,
		})

		return err
	}
}
