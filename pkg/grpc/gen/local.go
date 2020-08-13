// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"github.com/talos-systems/crypto/x509"
)

// LocalGenerator represents the OS identity generator.
type LocalGenerator struct {
	caKey []byte
	caCrt []byte
}

// NewLocalGenerator initializes a LocalGenerator.
func NewLocalGenerator(caKey, caCrt []byte) (g *LocalGenerator, err error) {
	g = &LocalGenerator{caKey, caCrt}

	return g, nil
}

// Identity creates an identity certificate using a local root CA.
func (g *LocalGenerator) Identity(csr *x509.CertificateSigningRequest) (ca, crt []byte, err error) {
	var c *x509.Certificate

	c, err = x509.NewCertificateFromCSRBytes(g.caCrt, g.caKey, csr.X509CertificateRequestPEM)
	if err != nil {
		return ca, crt, err
	}

	crt = c.X509CertificatePEM

	return g.caCrt, crt, nil
}
