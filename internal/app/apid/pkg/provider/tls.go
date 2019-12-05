// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package provider provides TLS config for client & server
package provider

import (
	stdlibtls "crypto/tls"
	"fmt"
	stdlibnet "net"
	"os"

	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/tls"
	"github.com/talos-systems/talos/pkg/net"
)

// TLSConfig provides client & server TLS configs for apid.
type TLSConfig struct {
	certificateProvider tls.CertificateProvider
}

// NewTLSConfig builds provider from configuration and endpoints.
func NewTLSConfig(config runtime.Configurator, endpoints []string) (*TLSConfig, error) {
	ips, err := net.IPAddrs()
	if err != nil {
		return nil, fmt.Errorf("failed to discover IP addresses: %w", err)
	}
	// TODO(andrewrynhard): Allow for DNS names.
	for _, san := range config.Machine().Security().CertSANs() {
		if ip := stdlibnet.ParseIP(san); ip != nil {
			ips = append(ips, ip)
		}
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to discover hostname: %w", err)
	}

	tlsConfig := &TLSConfig{}

	tlsConfig.certificateProvider, err = tls.NewRemoteRenewingFileCertificateProvider(
		config.Machine().Security().Token(),
		endpoints,
		constants.TrustdPort,
		hostname,
		ips,
	)
	if err != nil {
		return nil, err
	}

	return tlsConfig, nil
}

// ServerConfig generates server-side tls.Config.
func (tlsConfig *TLSConfig) ServerConfig() (*stdlibtls.Config, error) {
	ca, err := tlsConfig.certificateProvider.GetCA()
	if err != nil {
		return nil, fmt.Errorf("failed to get root CA: %w", err)
	}

	return tls.New(
		tls.WithClientAuthType(tls.Mutual),
		tls.WithCACertPEM(ca),
		tls.WithServerCertificateProvider(tlsConfig.certificateProvider),
	)
}

// ClientConfig generates client-side tls.Config.
func (tlsConfig *TLSConfig) ClientConfig() (*stdlibtls.Config, error) {
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
