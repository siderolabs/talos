// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"flag"
	"log"
	stdlibnet "net"
	"os"
	"regexp"
	"strings"

	"github.com/talos-systems/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/talos-systems/talos/internal/app/apid/pkg/backend"
	"github.com/talos-systems/talos/internal/app/apid/pkg/director"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/grpc/tls"
	"github.com/talos-systems/talos/pkg/net"
	"github.com/talos-systems/talos/pkg/startup"
)

var (
	configPath *string
	endpoints  *string
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	configPath = flag.String("config", "", "the path to the config")
	endpoints = flag.String("endpoints", "", "the IPs of the control plane nodes")

	flag.Parse()
}

func main() {
	if err := startup.RandSeed(); err != nil {
		log.Fatalf("failed to seed RNG: %v", err)
	}

	provider, err := createProvider()
	if err != nil {
		log.Fatalf("failed to create remote certificate provider: %+v", err)
	}

	ca, err := provider.GetCA()
	if err != nil {
		log.Fatalf("failed to get root CA: %+v", err)
	}

	tlsConfig, err := tls.New(
		tls.WithClientAuthType(tls.Mutual),
		tls.WithCACertPEM(ca),
		tls.WithServerCertificateProvider(provider),
	)
	if err != nil {
		log.Fatalf("failed to create OS-level TLS configuration: %v", err)
	}

	clientTLSConfig, err := tls.New(
		tls.WithClientAuthType(tls.Mutual),
		tls.WithCACertPEM(ca),
		tls.WithClientCertificateProvider(provider),
	)
	if err != nil {
		log.Fatalf("failed to create client TLS config: %v", err)
	}

	backendFactory := backend.NewAPIDFactory(clientTLSConfig)
	router := director.NewRouter(backendFactory.Get)

	router.RegisterLocalBackend("os.OS", backend.NewLocal("osd", constants.OSSocketPath))
	router.RegisterLocalBackend("machine.Machine", backend.NewLocal("machined", constants.MachineSocketPath))
	router.RegisterLocalBackend("time.Time", backend.NewLocal("timed", constants.TimeSocketPath))
	router.RegisterLocalBackend("network.Network", backend.NewLocal("networkd", constants.NetworkSocketPath))

	// all existing streaming methods
	for _, methodName := range []string{
		"/machine.Machine/CopyOut",
		"/machine.Machine/Kubeconfig",
		"/machine.Machine/LS",
		"/machine.Machine/Logs",
		"/machine.Machine/Read",
	} {
		router.RegisterStreamedRegex("^" + regexp.QuoteMeta(methodName) + "$")
	}

	// register future pattern: method should have suffix "Stream"
	router.RegisterStreamedRegex("Stream$")

	err = factory.ListenAndServe(
		router,
		factory.Port(constants.ApidPort),
		factory.WithDefaultLog(),
		factory.ServerOptions(
			grpc.Creds(
				credentials.NewTLS(tlsConfig),
			),
			grpc.CustomCodec(proxy.Codec()),
			grpc.UnknownServiceHandler(
				proxy.TransparentHandler(
					router.Director,
					proxy.WithStreamedDetector(router.StreamedDetector),
				)),
		),
	)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func createProvider() (tls.CertificateProvider, error) {
	content, err := config.FromFile(*configPath)
	if err != nil {
		log.Fatalf("open config: %v", err)
	}

	config, err := config.New(content)
	if err != nil {
		log.Fatalf("open config: %v", err)
	}

	ips, err := net.IPAddrs()
	if err != nil {
		log.Fatalf("failed to discover IP addresses: %+v", err)
	}
	// TODO(andrewrynhard): Allow for DNS names.
	for _, san := range config.Machine().Security().CertSANs() {
		if ip := stdlibnet.ParseIP(san); ip != nil {
			ips = append(ips, ip)
		}
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("failed to discover hostname: %+v", err)
	}

	return tls.NewRemoteRenewingFileCertificateProvider(config.Machine().Security().Token(), strings.Split(*endpoints, ","), constants.TrustdPort, hostname, ips)
}
