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
	"github.com/talos-systems/talos/pkg/grpc/middleware/auth/basic"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Run executes the configuration receiver, returning any configuration it receives.
func Run(ctx context.Context, logger *log.Logger, r runtime.Runtime) ([]byte, error) {
	ips, err := tnet.IPAddrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get list of IPs: %w", err)
	}

	tlsConfig, err := genTLSConfig(ips)
	if err != nil {
		return nil, err
	}

	cfgCh := make(chan []byte)

	s := server.New(r, logger, cfgCh)

	// Start the server.

	creds := basic.NewTokenCredentials("")

	server := factory.NewServer(
		s,
		factory.WithDefaultLog(),
		factory.WithUnaryInterceptor(creds.UnaryInterceptor()),
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
		// nolint: errcheck
		server.Serve(listener)
	}()

	logger.Println("this machine is reachable at:")

	for _, ip := range ips {
		logger.Printf("\t%s\n", ip.String())
	}

	select {
	case cfg := <-cfgCh:
		server.GracefulStop()

		return cfg, err
	case <-ctx.Done():
		return nil, fmt.Errorf("context is done")
	}
}

func genTLSConfig(ips []net.IP) (*tls.Config, error) {
	ca, err := x509.NewSelfSignedCertificateAuthority()
	if err != nil {
		return nil, fmt.Errorf("failed to generate self-signed CA: %w", err)
	}

	ips = append(ips, net.ParseIP("127.0.0.1"), net.ParseIP("::1"))

	dnsNames, err := tnet.DNSNames()
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS names: %w", err)
	}

	var generator ttls.Generator

	generator, err = gen.NewLocalGenerator(ca.KeyPEM, ca.CrtPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create local generator provider: %w", err)
	}

	var provider ttls.CertificateProvider

	provider, err = ttls.NewRenewingCertificateProvider(generator, dnsNames, ips)
	if err != nil {
		return nil, fmt.Errorf("failed to create local certificate provider: %w", err)
	}

	caProvider, err := provider.GetCA()
	if err != nil {
		return nil, fmt.Errorf("failed to get CA: %w", err)
	}

	tlsConfig, err := ttls.New(
		ttls.WithClientAuthType(ttls.ServerOnly),
		ttls.WithCACertPEM(caProvider),
		ttls.WithServerCertificateProvider(provider),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tlsconfig: %w", err)
	}

	return tlsConfig, nil
}
