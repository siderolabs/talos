// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package provider provides TLS config for client & server.
package provider

import (
	stdlibtls "crypto/tls"
	"fmt"
	"log"
	stdlibnet "net"
	"reflect"
	"sort"
	"time"

	"github.com/talos-systems/crypto/tls"
	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/net"

	"github.com/talos-systems/talos/pkg/grpc/gen"
	"github.com/talos-systems/talos/pkg/machinery/config"
)

// TLSConfig provides client & server TLS configs for apid.
type TLSConfig struct {
	endpoints           Endpoints
	lastEndpointList    []string
	generator           *gen.RemoteGenerator
	certificateProvider tls.CertificateProvider
}

// NewTLSConfig builds provider from configuration and endpoints.
func NewTLSConfig(config config.Provider, endpoints Endpoints) (*TLSConfig, error) {
	ips, err := net.IPAddrs()
	if err != nil {
		return nil, fmt.Errorf("failed to discover IP addresses: %w", err)
	}

	dnsNames, err := net.DNSNames()
	if err != nil {
		return nil, err
	}

	for _, san := range config.Machine().Security().CertSANs() {
		if ip := stdlibnet.ParseIP(san); ip != nil {
			ips = append(ips, ip)
		} else {
			dnsNames = append(dnsNames, san)
		}
	}

	endpointList, err := endpoints.GetEndpoints()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch initial endpoint list: %w", err)
	}

	sort.Strings(endpointList)

	tlsConfig := &TLSConfig{
		endpoints:        endpoints,
		lastEndpointList: endpointList,
	}

	tlsConfig.generator, err = gen.NewRemoteGenerator(
		config.Machine().Security().Token(),
		endpointList,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote certificate genertor: %w", err)
	}

	tlsConfig.certificateProvider, err = tls.NewRenewingCertificateProvider(
		tlsConfig.generator,
		dnsNames,
		ips,
	)
	if err != nil {
		return nil, err
	}

	go tlsConfig.refreshEndpoints()

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

func (tlsConfig *TLSConfig) refreshEndpoints() {
	// refresh endpoints 1/20 of the default certificate validity time
	ticker := time.NewTicker(x509.DefaultCertificateValidityDuration / 20)
	defer ticker.Stop()

	for {
		<-ticker.C

		endpointList, err := tlsConfig.endpoints.GetEndpoints()
		if err != nil {
			log.Printf("error refreshing endpoints: %s", err)

			continue
		}

		sort.Strings(endpointList)

		if reflect.DeepEqual(tlsConfig.lastEndpointList, endpointList) {
			continue
		}

		if err = tlsConfig.generator.SetEndpoints(endpointList); err != nil {
			log.Printf("error setting new endpoints %v: %s", endpointList, err)

			continue
		}

		tlsConfig.lastEndpointList = endpointList

		log.Printf("updated control plane endpoints to %v", endpointList)
	}
}
