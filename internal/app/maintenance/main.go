// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package maintenance

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"

	ttls "github.com/talos-systems/crypto/tls"
	"github.com/talos-systems/crypto/x509"
	tnet "github.com/talos-systems/net"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/maintenance/server"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/grpc/gen"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Run executes the configuration receiver, returning any configuration it receives.
func Run(ctx context.Context, logger *log.Logger, r runtime.Runtime) ([]byte, error) {
	ips, err := tnet.IPAddrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get list of IPs: %w", err)
	}

	tlsConfig, provider, err := genTLSConfig(ips)
	if err != nil {
		return nil, err
	}

	cert, err := provider.GetCertificate(nil)
	if err != nil {
		return nil, err
	}

	certFingerprint, err := x509.SPKIFingerprintFromDER(cert.Certificate[0])
	if err != nil {
		return nil, err
	}

	cfgCh := make(chan []byte)

	s := server.New(r, logger, cfgCh)

	// Start the server.
	server := factory.NewServer(
		s,
		factory.WithDefaultLog(),
		factory.ServerOptions(
			grpc.Creds(
				credentials.NewTLS(tlsConfig),
			),
		),
	)

	listener, err := factory.NewListener(factory.Port(constants.ApidPort))
	if err != nil {
		return nil, err
	}

	defer server.GracefulStop()

	go func() {
		//nolint:errcheck
		server.Serve(listener)
	}()

	logger.Println("this machine is reachable at:")

	for _, ip := range ips {
		logger.Printf("\t%s", ip.String())
	}

	firstIP := "<IP>"

	if len(ips) > 0 {
		firstIP = ips[0].String()
	}

	logger.Println("server certificate fingerprint:")
	logger.Printf("\t%s", certFingerprint)

	logger.Println()
	logger.Println("upload configuration using talosctl:")
	logger.Printf("\ttalosctl apply-config --insecure --nodes %s --file <config.yaml>", firstIP)
	logger.Println("or apply configuration using talosctl interactive installer:")
	logger.Printf("\ttalosctl apply-config --insecure --nodes %s --interactive", firstIP)
	logger.Println("optionally with node fingerprint check:")
	logger.Printf("\ttalosctl apply-config --insecure --nodes %s --cert-fingerprint '%s' --file <config.yaml>", firstIP, certFingerprint)

	select {
	case cfg := <-cfgCh:
		server.GracefulStop()

		return cfg, err
	case <-ctx.Done():
		return nil, fmt.Errorf("context is done")
	}
}

func genTLSConfig(ips []net.IP) (tlsConfig *tls.Config, provider ttls.CertificateProvider, err error) {
	ca, err := x509.NewSelfSignedCertificateAuthority()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate self-signed CA: %w", err)
	}

	ips = append(ips, net.ParseIP("127.0.0.1"), net.ParseIP("::1"))

	dnsNames, err := tnet.DNSNames()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get DNS names: %w", err)
	}

	var generator ttls.Generator

	generator, err = gen.NewLocalGenerator(ca.KeyPEM, ca.CrtPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create local generator provider: %w", err)
	}

	provider, err = ttls.NewRenewingCertificateProvider(generator, dnsNames, ips)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create local certificate provider: %w", err)
	}

	caCertPEM, err := provider.GetCA()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get CA: %w", err)
	}

	tlsConfig, err = ttls.New(
		ttls.WithClientAuthType(ttls.ServerOnly),
		ttls.WithCACertPEM(caCertPEM),
		ttls.WithServerCertificateProvider(provider),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate tlsconfig: %w", err)
	}

	return tlsConfig, provider, nil
}
