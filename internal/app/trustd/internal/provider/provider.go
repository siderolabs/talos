// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package provider provides TLS config for client & server.
package provider

import (
	"bytes"
	"context"
	stdlibtls "crypto/tls"
	stdx509 "crypto/x509"
	"fmt"
	"log"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/tls"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// TLSConfig provides client & server TLS configs for trustd.
type TLSConfig struct {
	certificateProvider *certificateProvider

	watchCh <-chan state.Event
}

// NewTLSConfig builds provider from configuration and endpoints.
func NewTLSConfig(ctx context.Context, resources state.State) (*TLSConfig, error) {
	watchCh := make(chan state.Event)

	if err := resources.Watch(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.TrustdType, secrets.TrustdID, resource.VersionUndefined), watchCh); err != nil {
		return nil, fmt.Errorf("error setting up watch: %w", err)
	}

	// wait for the first event to set up certificate provider
	provider := &certificateProvider{}

	for {
		var event state.Event

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event = <-watchCh:
		}

		switch event.Type {
		case state.Created, state.Updated:
			// expected
		case state.Destroyed, state.Bootstrapped:
			// ignore, we'll get another event
			continue
		case state.Errored:
			return nil, fmt.Errorf("error watching for trustd certificates: %w", event.Error)
		}

		trustdCerts := event.Resource.(*secrets.Trustd) //nolint:errcheck,forcetypeassert

		if err := provider.Update(trustdCerts); err != nil {
			return nil, err
		}

		return &TLSConfig{
			certificateProvider: provider,
			watchCh:             watchCh,
		}, nil
	}
}

// Watch for updates to trustd certificates.
func (tlsConfig *TLSConfig) Watch(ctx context.Context) error {
	for {
		var event state.Event

		select {
		case <-ctx.Done():
			return nil
		case event = <-tlsConfig.watchCh:
		}

		switch event.Type {
		case state.Created, state.Updated:
			// expected
		case state.Destroyed, state.Bootstrapped:
			// ignore, we'll get another event
			continue
		case state.Errored:
			log.Printf("error watching for trustd certificates: %s", event.Error)
		}

		trustdCerts := event.Resource.(*secrets.Trustd) //nolint:errcheck,forcetypeassert

		if err := tlsConfig.certificateProvider.Update(trustdCerts); err != nil {
			return fmt.Errorf("failed updating cert: %w", err)
		}
	}
}

// ServerConfig generates server-side tls.Config.
func (tlsConfig *TLSConfig) ServerConfig() (*stdlibtls.Config, error) {
	ca, err := tlsConfig.certificateProvider.GetCA()
	if err != nil {
		return nil, fmt.Errorf("failed to get root CA: %w", err)
	}

	return tls.New(
		tls.WithClientAuthType(tls.ServerOnly),
		tls.WithCACertPEM(ca),
		tls.WithServerCertificateProvider(tlsConfig.certificateProvider),
	)
}

type certificateProvider struct {
	mu sync.Mutex

	ca         []byte
	caCertPool *stdx509.CertPool

	serverCert *stdlibtls.Certificate
}

func (p *certificateProvider) Update(trustdCerts *secrets.Trustd) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ca = bytes.Join(
		xslices.Map(
			trustdCerts.TypedSpec().AcceptedCAs,
			func(cert *x509.PEMEncodedCertificate) []byte {
				return cert.Crt
			},
		),
		nil,
	)

	p.caCertPool = stdx509.NewCertPool()
	if !p.caCertPool.AppendCertsFromPEM(p.ca) {
		return fmt.Errorf("failed to parse root CA")
	}

	serverCert, err := stdlibtls.X509KeyPair(trustdCerts.TypedSpec().Server.Crt, trustdCerts.TypedSpec().Server.Key)
	if err != nil {
		return fmt.Errorf("failed to parse server cert and key into a TLS Certificate: %w", err)
	}

	p.serverCert = &serverCert

	return nil
}

func (p *certificateProvider) GetCA() ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.ca, nil
}

func (p *certificateProvider) GetCACertPool() (*stdx509.CertPool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.caCertPool, nil
}

func (p *certificateProvider) GetCertificate(h *stdlibtls.ClientHelloInfo) (*stdlibtls.Certificate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.serverCert, nil
}

func (p *certificateProvider) GetClientCertificate(*stdlibtls.CertificateRequestInfo) (*stdlibtls.Certificate, error) {
	return nil, nil
}
