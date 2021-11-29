// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package trustd

import (
	"context"
	"flag"
	"log"
	stdlibnet "net"

	"github.com/talos-systems/crypto/tls"
	"github.com/talos-systems/crypto/x509"
	debug "github.com/talos-systems/go-debug"
	"github.com/talos-systems/net"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/talos-systems/talos/internal/app/trustd/internal/reg"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/grpc/gen"
	"github.com/talos-systems/talos/pkg/grpc/middleware/auth/basic"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	"github.com/talos-systems/talos/pkg/startup"
)

func runDebugServer(ctx context.Context) {
	const debugAddr = ":9983"

	debugLogFunc := func(msg string) {
		log.Print(msg)
	}

	if err := debug.ListenAndServe(ctx, debugAddr, debugLogFunc); err != nil {
		log.Fatalf("failed to start debug server: %s", err)
	}
}

// Main is the entrypoint into trustd.
//
//nolint:gocyclo
func Main() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	flag.Parse()

	go runDebugServer(context.TODO())

	var err error

	if err = startup.RandSeed(); err != nil {
		log.Fatalf("startup: %s", err)
	}

	config, err := configloader.NewFromStdin()
	if err != nil {
		log.Fatal(err)
	}

	ips, err := net.IPAddrs()
	if err != nil {
		log.Fatal(err)
	}

	ips = net.IPFilter(ips, network.NotSideroLinkStdIP)

	dnsNames, err := net.DNSNames()
	if err != nil {
		log.Fatal(err)
	}

	for _, san := range config.Machine().Security().CertSANs() {
		if ip := stdlibnet.ParseIP(san); ip != nil {
			ips = append(ips, ip)
		} else {
			dnsNames = append(dnsNames, san)
		}
	}

	var generator tls.Generator

	generator, err = gen.NewLocalGenerator(config.Machine().Security().CA().Key, config.Machine().Security().CA().Crt)
	if err != nil {
		log.Fatalln("failed to create local generator provider:", err)
	}

	var provider tls.CertificateProvider

	provider, err = tls.NewRenewingCertificateProvider(generator, x509.DNSNames(dnsNames), x509.IPAddresses(ips))
	if err != nil {
		log.Fatalln("failed to create local certificate provider:", err)
	}

	ca, err := provider.GetCA()
	if err != nil {
		log.Fatal(err)
	}

	tlsConfig, err := tls.New(
		tls.WithClientAuthType(tls.ServerOnly),
		tls.WithCACertPEM(ca),
		tls.WithServerCertificateProvider(provider),
	)
	if err != nil {
		log.Fatalf("failed to create TLS config: %v", err)
	}

	creds := basic.NewTokenCredentials(config.Machine().Security().Token())

	err = factory.ListenAndServe(
		&reg.Registrator{Config: config},
		factory.Port(constants.TrustdPort),
		factory.WithDefaultLog(),
		factory.WithUnaryInterceptor(creds.UnaryInterceptor()),
		factory.ServerOptions(
			grpc.Creds(
				credentials.NewTLS(tlsConfig),
			),
		),
	)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
}
