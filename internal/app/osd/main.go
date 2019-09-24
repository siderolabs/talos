/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"log"
	stdlibnet "net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/talos-systems/talos/internal/app/osd/internal/reg"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/grpc/tls"
	"github.com/talos-systems/talos/pkg/net"
	"github.com/talos-systems/talos/pkg/startup"
	"github.com/talos-systems/talos/pkg/userdata"
)

var dataPath *string

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	dataPath = flag.String("userdata", "", "the path to the user data")
	flag.Parse()
}

// nolint: gocyclo
func main() {
	if err := startup.RandSeed(); err != nil {
		log.Fatalf("failed to seed RNG: %s", err)
	}

	data, err := userdata.Open(*dataPath)
	if err != nil {
		log.Fatalf("open user data: %v", err)
	}

	ips, err := net.IPAddrs()
	if err != nil {
		log.Fatalf("failed to discover IP addresses: %+v", err)
	}
	// TODO(andrewrynhard): Allow for DNS names.
	for _, san := range data.Services.Trustd.CertSANs {
		if ip := stdlibnet.ParseIP(san); ip != nil {
			ips = append(ips, ip)
		}
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("failed to discover hostname: %+v", err)
	}

	var provider tls.CertificateProvider
	provider, err = tls.NewRemoteRenewingFileCertificateProvider(data.Services.Trustd.Token, data.Services.Trustd.Endpoints, constants.TrustdPort, hostname, ips)
	if err != nil {
		log.Fatalf("failed to create remote certificate provider: %+v", err)
	}

	ca, err := provider.GetCA()
	if err != nil {
		log.Fatalf("failed to get root CA: %+v", err)
	}

	config, err := tls.New(
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
			Data:          data,
			MachineClient: machineClient,
			TimeClient:    timeClient,
			NetworkClient: networkClient,
		},
		factory.Port(constants.OsdPort),
		factory.ServerOptions(
			grpc.Creds(
				credentials.NewTLS(config),
			),
		),
	)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
}
