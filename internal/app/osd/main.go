/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"log"
	stdlibnet "net"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/talos-systems/talos/internal/app/osd/internal/reg"
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

// nolint: gocyclo
func main() {
	if err := startup.RandSeed(); err != nil {
		log.Fatalf("failed to seed RNG: %s", err)
	}

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

	var provider tls.CertificateProvider
	provider, err = tls.NewRemoteRenewingFileCertificateProvider(config.Machine().Security().Token(), strings.Split(*endpoints, ","), constants.TrustdPort, hostname, ips)
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
		tls.WithCertificateProvider(provider),
	)
	if err != nil {
		log.Fatalf("failed to create OS-level TLS configuration: %v", err)
	}

	machineClient, err := reg.NewMachineClient()
	if err != nil {
		log.Fatalf("init client: %v", err)
	}

	timeClient, err := reg.NewTimeClient()
	if err != nil {
		log.Fatalf("ntp client: %v", err)
	}

	networkClient, err := reg.NewNetworkClient()
	if err != nil {
		log.Fatalf("networkd client: %v", err)
	}

	err = factory.ListenAndServe(
		&reg.Registrator{
			MachineClient: machineClient,
			TimeClient:    timeClient,
			NetworkClient: networkClient,
		},
		factory.Port(constants.OsdPort),
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
