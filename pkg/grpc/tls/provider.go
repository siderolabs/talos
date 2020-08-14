// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	talosx509 "github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/constants"
)

// CertificateProvider describes an interface by which TLS certificates may be managed.
type CertificateProvider interface {
	// GetCA returns the active root CA.
	GetCA() ([]byte, error)

	// GetCertificate returns the current certificate matching the given client request.
	GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error)

	// GetClientCertificate returns the current certificate to present to the server.
	GetClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error)
}

// Generator describes an interface to sign the CSR.
type Generator interface {
	Identity(csr *talosx509.CertificateSigningRequest) (ca, crt []byte, err error)
}

type certificateProvider struct {
	sync.RWMutex

	generator Generator

	ca  []byte
	crt *tls.Certificate

	dnsNames []string
	ips      []net.IP
}

// NewRenewingCertificateProvider returns a new CertificateProvider
// which manages and updates its certificates using Generator.
func NewRenewingCertificateProvider(generator Generator, dnsNames []string, ips []net.IP) (CertificateProvider, error) {
	provider := &certificateProvider{
		generator: generator,
		dnsNames:  dnsNames,
		ips:       ips,
	}

	var (
		ca   []byte
		cert tls.Certificate
		err  error
	)

	if ca, cert, err = provider.update(); err != nil {
		return nil, fmt.Errorf("failed to create initial certificate: %w", err)
	}

	provider.updateCertificates(ca, &cert)

	// nolint: errcheck
	go provider.manageUpdates(context.Background())

	return provider, nil
}

func (p *certificateProvider) update() (ca []byte, cert tls.Certificate, err error) {
	var (
		crt      []byte
		csr      *talosx509.CertificateSigningRequest
		identity *talosx509.PEMEncodedCertificateAndKey
	)

	csr, identity, err = talosx509.NewCSRAndIdentity(p.dnsNames, p.ips)
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

func (p *certificateProvider) GetCA() ([]byte, error) {
	if p == nil {
		return nil, errors.New("no provider")
	}

	p.RLock()
	defer p.RUnlock()

	return p.ca, nil
}

func (p *certificateProvider) GetCertificate(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if p == nil {
		return nil, errors.New("no provider")
	}

	p.RLock()
	defer p.RUnlock()

	return p.crt, nil
}

func (p *certificateProvider) GetClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	return p.GetCertificate(nil)
}

func (p *certificateProvider) updateCertificates(ca []byte, cert *tls.Certificate) {
	p.Lock()
	defer p.Unlock()

	p.ca = ca
	p.crt = cert
}

func (p *certificateProvider) manageUpdates(ctx context.Context) (err error) {
	nextRenewal := constants.DefaultCertificateValidityDuration

	for ctx.Err() == nil {
		// nolint: errcheck
		if c, _ := p.GetCertificate(nil); c != nil {
			if len(c.Certificate) > 0 {
				var crt *x509.Certificate
				crt, err = x509.ParseCertificate(c.Certificate[0])

				if err == nil {
					nextRenewal = time.Until(crt.NotAfter) / 2
				} else {
					log.Println("failed to parse current leaf certificate")
				}
			} else {
				log.Println("current leaf certificate not found")
			}
		} else {
			log.Println("certificate not found")
		}

		log.Println("next renewal in", nextRenewal)

		if nextRenewal > constants.DefaultCertificateValidityDuration {
			nextRenewal = constants.DefaultCertificateValidityDuration
		}

		select {
		case <-time.After(nextRenewal):
		case <-ctx.Done():
			return nil
		}

		var (
			ca   []byte
			cert tls.Certificate
		)

		if ca, cert, err = p.update(); err != nil {
			log.Println("failed to renew certificate:", err)
			continue
		}

		p.updateCertificates(ca, &cert)
	}

	return errors.New("certificate update manager exited unexpectedly")
}
