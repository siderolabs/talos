// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tls

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/grpc/gen"
)

type renewingRemoteCertificateProvider struct {
	embeddableCertificateProvider

	generator *gen.RemoteGenerator
}

// NewRemoteRenewingFileCertificateProvider returns a new CertificateProvider
// which manages and updates its certificates from the security API.
func NewRemoteRenewingFileCertificateProvider(token string, endpoints []string, port int, dnsNames []string, ips []net.IP) (CertificateProvider, error) {
	g, err := gen.NewRemoteGenerator(token, endpoints, port)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS generator: %w", err)
	}

	provider := &renewingRemoteCertificateProvider{
		generator: g,
	}

	provider.embeddableCertificateProvider = embeddableCertificateProvider{
		dnsNames:   dnsNames,
		ips:        ips,
		updateFunc: provider.update,
	}

	var (
		ca   []byte
		cert tls.Certificate
	)

	if ca, cert, err = provider.updateFunc(); err != nil {
		return nil, fmt.Errorf("failed to create initial certificate: %w", err)
	}

	if err = provider.UpdateCertificates(ca, &cert); err != nil {
		return nil, err
	}

	// nolint: errcheck
	go provider.manageUpdates(context.Background())

	return provider, nil
}

// nolint: dupl
func (p *renewingRemoteCertificateProvider) update() (ca []byte, cert tls.Certificate, err error) {
	var (
		crt      []byte
		csr      *x509.CertificateSigningRequest
		identity *x509.PEMEncodedCertificateAndKey
	)

	csr, identity, err = x509.NewCSRAndIdentity(p.dnsNames, p.ips)
	if err != nil {
		return nil, cert, err
	}

	if ca, crt, err = p.generator.Identity(csr); err != nil {
		return nil, cert, fmt.Errorf("failed to generate identity: %w", err)
	}

	identity.Crt = crt

	cert, err = tls.X509KeyPair(identity.Crt, identity.Key)
	if err != nil {
		return nil, cert, fmt.Errorf("failed to parse cert and key into a TLS Certificate: %w", err)
	}

	return ca, cert, nil
}
