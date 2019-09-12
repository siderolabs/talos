/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"context"
	"flag"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/talos-systems/talos/internal/app/osd/internal/reg"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/grpc/tls"
	"github.com/talos-systems/talos/pkg/startup"
	"github.com/talos-systems/talos/pkg/userdata"
)

var dataPath *string

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	dataPath = flag.String("userdata", "", "the path to the user data")
	flag.Parse()
}

func main() {
	if err := startup.RandSeed(); err != nil {
		log.Fatalf("startup: %s", err)
	}

	data, err := userdata.Open(*dataPath)
	if err != nil {
		log.Fatalf("open user data: %v", err)
	}

	tlsCertProvider, err := tls.NewRenewingFileCertificateProvider(context.TODO(), data)
	if err != nil {
		log.Fatalln("failed to create new dynamic certificate provider:", err)
	}
	config, err := tls.NewConfigWithOpts(
		tls.WithClientAuthType(tls.Mutual),
		tls.WithCACertPEM(data.Security.OS.CA.Crt),
		tls.WithCertificateProvider(tlsCertProvider))
	if err != nil {
		log.Fatalf("failed to create OS-level TLS configuration: %v", err)
	}

	initClient, err := reg.NewInitServiceClient()
	if err != nil {
		log.Fatalf("init client: %v", err)
	}

	ntpdClient, err := reg.NewNtpdClient()
	if err != nil {
		log.Fatalf("ntp client: %v", err)
	}

	networkdClient, err := reg.NewNetworkdClient()
	if err != nil {
		log.Fatalf("networkd client: %v", err)
	}

	log.Println("Starting osd")
	err = factory.ListenAndServe(
		&reg.Registrator{
			Data:              data,
			InitServiceClient: initClient,
			NtpdClient:        ntpdClient,
			NetworkdClient:    networkdClient,
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
