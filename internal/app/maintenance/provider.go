// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package maintenance

import (
	stdlibtls "crypto/tls"
	"crypto/x509"
	"fmt"
	"sync/atomic"

	"github.com/siderolabs/crypto/tls"

	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// NewTLSProvider creates a new TLS provider for maintenance service.
//
// The provider expects that the certificates are pushed to it.
func NewTLSProvider() *TLSProvider {
	return &TLSProvider{}
}

// TLSProvider provides TLS configuration for maintenance service.
type TLSProvider struct {
	serverCert atomic.Pointer[stdlibtls.Certificate]
}

// TLSConfig generates server-side tls.Config.
func (provider *TLSProvider) TLSConfig() (*stdlibtls.Config, error) {
	return tls.New(
		tls.WithClientAuthType(tls.ServerOnly),
		tls.WithServerCertificateProvider(provider),
	)
}

// Update the certificate in the provider.
func (provider *TLSProvider) Update(maintenanceCerts *secrets.MaintenanceServiceCerts) error {
	serverCert, err := stdlibtls.X509KeyPair(maintenanceCerts.TypedSpec().Server.Crt, maintenanceCerts.TypedSpec().Server.Key)
	if err != nil {
		return fmt.Errorf("failed to parse server cert and key into a TLS Certificate: %w", err)
	}

	provider.serverCert.Store(&serverCert)

	return nil
}

// GetCA implements tls.CertificateProvider interface.
func (provider *TLSProvider) GetCA() ([]byte, error) {
	return nil, nil
}

// GetCACertPool implements tls.CertificateProvider interface.
func (provider *TLSProvider) GetCACertPool() (*x509.CertPool, error) {
	return nil, nil
}

// GetCertificate implements tls.CertificateProvider interface.
func (provider *TLSProvider) GetCertificate(h *stdlibtls.ClientHelloInfo) (*stdlibtls.Certificate, error) {
	return provider.serverCert.Load(), nil
}

// GetClientCertificate implements tls.CertificateProvider interface.
func (provider *TLSProvider) GetClientCertificate(*stdlibtls.CertificateRequestInfo) (*stdlibtls.Certificate, error) {
	return nil, nil
}
