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

	"github.com/cosi-project/runtime/pkg/resource"
	ttls "github.com/talos-systems/crypto/tls"
	"github.com/talos-systems/crypto/x509"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/maintenance/server"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/grpc/gen"
	"github.com/talos-systems/talos/pkg/grpc/middleware/authz"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// Run executes the configuration receiver, returning any configuration it receives.
//
//nolint:gocyclo
func Run(ctx context.Context, logger *log.Logger, r runtime.Runtime) ([]byte, error) {
	logger.Println("waiting for network address and hostname to be ready")

	if err := network.NewReadyCondition(r.State().V1Alpha2().Resources(), network.AddressReady, network.HostnameReady).Wait(ctx); err != nil {
		return nil, fmt.Errorf("error waiting for the network to be ready: %w", err)
	}

	currentAddresses, err := r.State().V1Alpha2().Resources().Get(ctx, resource.NewMetadata(network.NamespaceName, network.NodeAddressType, network.NodeAddressCurrentID, resource.VersionUndefined))
	if err != nil {
		return nil, fmt.Errorf("error getting node addresses: %w", err)
	}

	ips := currentAddresses.(*network.NodeAddress).TypedSpec().IPs()

	hostnameStatus, err := r.State().V1Alpha2().Resources().Get(ctx, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined))
	if err != nil {
		return nil, fmt.Errorf("error getting node hostname: %w", err)
	}

	dnsNames := hostnameStatus.(*network.HostnameStatus).TypedSpec().DNSNames()

	tlsConfig, provider, err := genTLSConfig(ips, dnsNames)
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

	injector := &authz.Injector{
		Mode:   authz.ReadOnly,
		Logger: log.New(logger.Writer(), "machined/authz/injector ", log.Flags()).Printf,
	}

	// Start the server.
	server := factory.NewServer(
		s,
		factory.WithDefaultLog(),
		factory.ServerOptions(
			grpc.Creds(
				credentials.NewTLS(tlsConfig),
			),
		),

		factory.WithUnaryInterceptor(injector.UnaryInterceptor()),
		factory.WithStreamInterceptor(injector.StreamInterceptor()),
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
	logger.Printf("\ttalosctl apply-config --insecure --nodes %s --mode=interactive", firstIP)
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

func genTLSConfig(ips []netaddr.IP, dnsNames []string) (tlsConfig *tls.Config, provider ttls.CertificateProvider, err error) {
	ca, err := x509.NewSelfSignedCertificateAuthority()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate self-signed CA: %w", err)
	}

	ips = append(ips, netaddr.MustParseIP("127.0.0.1"), netaddr.MustParseIP("::1"))

	netIPs := make([]net.IP, len(ips))

	for i := range netIPs {
		netIPs[i] = ips[i].IPAddr().IP
	}

	var generator ttls.Generator

	generator, err = gen.NewLocalGenerator(ca.KeyPEM, ca.CrtPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create local generator provider: %w", err)
	}

	provider, err = ttls.NewRenewingCertificateProvider(generator, x509.DNSNames(dnsNames), x509.IPAddresses(netIPs))
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
