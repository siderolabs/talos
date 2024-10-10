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
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/tls"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// TLSConfig provides client & server TLS configs for apid.
type TLSConfig struct {
	certificateProvider *certificateProvider
	watchCh             <-chan state.Event
}

// NewTLSConfig builds provider from configuration and endpoints.
func NewTLSConfig(ctx context.Context, resources state.State) (*TLSConfig, error) {
	watchCh := make(chan state.Event)

	if err := resources.Watch(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.APIType, secrets.APIID, resource.VersionUndefined), watchCh); err != nil {
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
			return nil, fmt.Errorf("error watching for API certificates: %w", event.Error)
		}

		apiCerts := event.Resource.(*secrets.API) //nolint:errcheck,forcetypeassert

		if err := provider.Update(apiCerts); err != nil {
			return nil, err
		}

		return &TLSConfig{
			certificateProvider: provider,
			watchCh:             watchCh,
		}, nil
	}
}

// Watch for changes in API certificates and updates the TLSConfig.
func (tlsConfig *TLSConfig) Watch(ctx context.Context, onUpdate func()) error {
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
			return fmt.Errorf("error watching API certificates: %w", event.Error)
		}

		apiCerts := event.Resource.(*secrets.API) //nolint:errcheck,forcetypeassert

		if err := tlsConfig.certificateProvider.Update(apiCerts); err != nil {
			return fmt.Errorf("failed updating cert: %v", err)
		}

		if onUpdate != nil {
			onUpdate()
		}
	}
}

// ServerConfig generates server-side tls.Config.
func (tlsConfig *TLSConfig) ServerConfig() (*stdlibtls.Config, error) {
	return tls.New(
		tls.WithClientAuthType(tls.Mutual),
		tls.WithDynamicClientCA(tlsConfig.certificateProvider),
		tls.WithServerCertificateProvider(tlsConfig.certificateProvider),
	)
}

// ClientConfig generates client-side tls.Config.
func (tlsConfig *TLSConfig) ClientConfig() (*stdlibtls.Config, error) {
	if !tlsConfig.certificateProvider.HasClientCertificate() {
		return nil, nil
	}

	ca, err := tlsConfig.certificateProvider.GetCA()
	if err != nil {
		return nil, fmt.Errorf("failed to get root CA: %w", err)
	}

	return tls.New(
		tls.WithClientAuthType(tls.Mutual),
		tls.WithCACertPEM(ca),
		tls.WithClientCertificateProvider(tlsConfig.certificateProvider),
	)
}

type certificateProvider struct {
	mu sync.Mutex

	ca                     []byte
	caCertPool             *stdx509.CertPool
	clientCert, serverCert *stdlibtls.Certificate
}

func (p *certificateProvider) Update(apiCerts *secrets.API) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	serverCert, err := stdlibtls.X509KeyPair(apiCerts.TypedSpec().Server.Crt, apiCerts.TypedSpec().Server.Key)
	if err != nil {
		return fmt.Errorf("failed to parse server cert and key into a TLS Certificate: %w", err)
	}

	p.serverCert = &serverCert

	p.ca = bytes.Join(
		xslices.Map(
			apiCerts.TypedSpec().AcceptedCAs,
			func(cert *x509.PEMEncodedCertificate) []byte {
				return cert.Crt
			},
		),
		nil,
	)

	p.caCertPool = stdx509.NewCertPool()
	if !p.caCertPool.AppendCertsFromPEM(p.ca) {
		return fmt.Errorf("failed to parse CA certs into a CertPool")
	}

	if apiCerts.TypedSpec().Client != nil {
		clientCert, err := stdlibtls.X509KeyPair(apiCerts.TypedSpec().Client.Crt, apiCerts.TypedSpec().Client.Key)
		if err != nil {
			return fmt.Errorf("failed to parse client cert and key into a TLS Certificate: %w", err)
		}

		p.clientCert = &clientCert
	} else {
		p.clientCert = nil
	}

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

func (p *certificateProvider) GetCertificate(*stdlibtls.ClientHelloInfo) (*stdlibtls.Certificate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.serverCert, nil
}

func (p *certificateProvider) HasClientCertificate() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.clientCert != nil
}

func (p *certificateProvider) GetClientCertificate(*stdlibtls.CertificateRequestInfo) (*stdlibtls.Certificate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.clientCert, nil
}
