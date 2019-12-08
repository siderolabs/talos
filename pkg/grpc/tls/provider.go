// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log"
	"net"
	"sync"
	"time"

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

	// UpdateCertificate updates the stored certificate for the given client request.
	UpdateCertificates([]byte, *tls.Certificate) error
}

type embeddableCertificateProvider struct {
	sync.RWMutex

	ca  []byte
	crt *tls.Certificate

	dnsNames []string
	ips      []net.IP

	updateFunc  func() ([]byte, tls.Certificate, error)
	updateHooks []func(newCert *tls.Certificate)
}

func (p *embeddableCertificateProvider) GetCA() ([]byte, error) {
	if p == nil {
		return nil, errors.New("no provider")
	}

	p.RLock()
	defer p.RUnlock()

	return p.ca, nil
}

func (p *embeddableCertificateProvider) GetCertificate(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if p == nil {
		return nil, errors.New("no provider")
	}

	p.RLock()
	defer p.RUnlock()

	return p.crt, nil
}

func (p *embeddableCertificateProvider) GetClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	return p.GetCertificate(nil)
}

func (p *embeddableCertificateProvider) UpdateCertificates(ca []byte, cert *tls.Certificate) error {
	p.Lock()
	p.ca = ca
	p.crt = cert
	p.Unlock()

	for _, f := range p.updateHooks {
		f(cert)
	}

	return nil
}

func (p *embeddableCertificateProvider) manageUpdates(ctx context.Context) (err error) {
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

		if ca, cert, err = p.updateFunc(); err != nil {
			log.Println("failed to renew certificate:", err)
			continue
		}

		if err = p.UpdateCertificates(ca, &cert); err != nil {
			log.Println("failed to renew certificate:", err)
			continue
		}
	}

	return errors.New("certificate update manager exited unexpectedly")
}
