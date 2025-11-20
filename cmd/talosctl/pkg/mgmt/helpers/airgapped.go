// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	stdx509 "crypto/x509"
	"net"

	"github.com/siderolabs/crypto/x509"
)

// GenerateSelfSignedCert generates self-signed certificate.
func GenerateSelfSignedCert(sanIPs []net.IP, sanNames []string) ([]byte, []byte, []byte, error) {
	ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true))
	if err != nil {
		return nil, nil, nil, err
	}

	serverIdentity, err := x509.NewKeyPair(ca,
		x509.Organization("test"),
		x509.CommonName("server"),
		x509.IPAddresses(sanIPs),
		x509.DNSNames(sanNames),
		x509.ExtKeyUsage([]stdx509.ExtKeyUsage{stdx509.ExtKeyUsageServerAuth}),
	)
	if err != nil {
		return nil, nil, nil, err
	}

	return ca.CrtPEM, serverIdentity.CrtPEM, serverIdentity.KeyPEM, nil
}
