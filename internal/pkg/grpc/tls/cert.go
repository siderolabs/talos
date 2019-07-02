/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/grpc/gen"
	"github.com/talos-systems/talos/pkg/userdata"
)

/*
// minFileCacheInterval is the minimum amount of time a file-based cert is presumed to be good, before re-checking the file
var minFileCacheInterval = time.Minute

// maxFileCacheInterval is the maximum amount of time a file-based cert is presumed to be good, before re-checking the file
var maxFileCacheInterval = time.Hour
*/

// CertificateProvider describes an interface by which TLS certificates may be managed
type CertificateProvider interface {

	// GetCertificate returns the current certificate matching the given client request
	GetCertificate(h *tls.ClientHelloInfo) (*tls.Certificate, error)

	// UpdateCertificate updates the stored certificate for the given client request
	UpdateCertificate(h *tls.ClientHelloInfo, cert *tls.Certificate) error
}

type singleCertificateProvider struct {
	sync.RWMutex
	cert *tls.Certificate

	updateHooks []func(newCert *tls.Certificate)
}

func (p *singleCertificateProvider) GetCertificate(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if p == nil {
		return nil, errors.New("no provider")
	}

	p.RLock()
	defer p.RUnlock()
	return p.cert, nil
}

func (p *singleCertificateProvider) UpdateCertificate(h *tls.ClientHelloInfo, cert *tls.Certificate) error {
	p.Lock()
	p.cert = cert
	p.Unlock()

	for _, f := range p.updateHooks {
		f(cert)
	}
	return nil
}

type userDataCertificateProvider struct {
	data *userdata.OSSecurity
}

func (p *userDataCertificateProvider) GetCertificate(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert, err := tls.X509KeyPair(p.data.Identity.Crt, p.data.Identity.Key)
	return &cert, err
}

func (p *userDataCertificateProvider) UpdateCertificate(h *tls.ClientHelloInfo, cert *tls.Certificate) error {
	// No-op
	return nil
}

/*
type fileCertificateProvider struct {
	singleCertificateProvider

	certFile, keyFile string
}

func (p *fileCertificateProvider) watchFiles(keyFile, certFile string) {
	var nextCheck, certCheck time.Duration
	for {

		nextCheck = maxFileCacheInterval
		if p.cert != nil && p.cert.Leaf != nil {
			if certCheck = time.Until(p.cert.Leaf.NotAfter) / 2; certCheck < nextCheck {
				nextCheck = certCheck
			}
		}
		if nextCheck < minFileCacheInterval {
			nextCheck = minFileCacheInterval
		}

		<-time.After(nextCheck)
		if err := p.loadKeyPair(); err != nil {
			log.Println("failed to load keypair:", err)
			continue
		}
	}
}

func (p *fileCertificateProvider) loadKeyPair() error {
	c, err := tls.LoadX509KeyPair(p.certFile, p.keyFile)
	if err != nil {
		return errors.Wrapf(err, "failed to load key pair (%s, %s)", p.certFile, p.keyFile)
	}

	return p.UpdateCertificate(nil, &c)
}
*/

type renewingFileCertificateProvider struct {
	singleCertificateProvider

	certFile string

	// For now, this is using the complete userdata object.  It should probably
	// be pared down to just what is necessary, later.  However, the current TLS
	// generator code requires the whole thing... so we do, too.
	data *userdata.UserData

	g *gen.Generator
}

// NewRenewingFileCertificateProvider returns a new CertificateProvider which
// manages and updates its certificates from trustd, storing a cache file copy.
//
// TODO: the flow here is a bit wonky, but it should be fixable after we change
// to have the default be ephemeral node certificates.  Until then, however, we
// are doing a dance between the userdata-stored cert, the filesystem-cached
// cert, and the memory-cached cert.
func NewRenewingFileCertificateProvider(ctx context.Context, data *userdata.UserData) (CertificateProvider, error) {
	g, err := gen.NewGenerator(data, constants.TrustdPort)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create TLS generator")
	}

	p := &renewingFileCertificateProvider{
		g:        g,
		data:     data,
		certFile: constants.NodeCertFile,
	}

	if err = p.loadInitialCert(); err != nil {
		return nil, errors.Wrap(err, "failed to load initial certificate")
	}

	go p.manageUpdates(ctx)

	return p, nil
}

func (p *renewingFileCertificateProvider) loadInitialCert() error {

	// TODO: eventually, we will reverse this priority, and have the override
	// come from the userdata.  For now, however, we use the local file to
	// override the userdata, because we _always_ have userdata certs, and the
	// userdata is intended to be immutable.

	data, err := ioutil.ReadFile(p.certFile)
	if err != nil || len(data) == 0 {
		// If we cannot read the cert from the file, then we use the userdata-supplied one
		data = p.data.Security.OS.Identity.Crt
	}

	cert, err := tls.X509KeyPair(data, p.data.Security.OS.Identity.Key)
	if err != nil {
		return errors.Wrap(err, "failed to parse cert and key into a TLS Certificate")
	}

	return p.UpdateCertificate(nil, &cert)
}

func (p *renewingFileCertificateProvider) manageUpdates(ctx context.Context) {
	var nextRenewal = constants.NodeCertRenewalInterval

	for ctx.Err() == nil {
		if c, _ := p.GetCertificate(nil); c != nil { // nolint: errcheck
			if len(c.Certificate) > 0 {
				cert, err := x509.ParseCertificate(c.Certificate[0])
				if err == nil {
					nextRenewal = time.Until(cert.NotAfter) / 2
				} else {
					log.Println("failed to parse current leaf certificate")
				}
			} else {
				log.Println("current leaf certificate not found")
			}
		} else {
			log.Println("certificate not found")
		}

		if nextRenewal > constants.NodeCertRenewalInterval {
			nextRenewal = constants.NodeCertRenewalInterval
		}

		select {
		case <-time.After(nextRenewal):
		case <-ctx.Done():
			return
		}
		if err := p.renewCert(); err != nil {
			log.Println("failed to renew certificate:", err)
			continue
		}
	}
}

func (p *renewingFileCertificateProvider) renewCert() error {
	if err := p.g.Identity(p.data); err != nil {
		return errors.Wrap(err, "failed to renew certificate")
	}

	// TODO: updating the cert using the generator automatically stores the new
	// cert to userdata.  Therefore, we need to pull that cert out in order to
	// update the CertificateProvider's cache of it
	cert, err := tls.X509KeyPair(p.data.Security.OS.Identity.Crt, p.data.Security.OS.Identity.Key)
	if err != nil {
		return errors.Wrap(err, "failed to parse cert and key into a TLS Certificate")
	}

	return p.UpdateCertificate(nil, &cert)
}
