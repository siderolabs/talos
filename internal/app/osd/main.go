/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"context"
	"flag"
	"log"

	"github.com/talos-systems/talos/internal/app/osd/internal/reg"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/grpc/factory"
	"github.com/talos-systems/talos/internal/pkg/grpc/tls"
	"github.com/talos-systems/talos/internal/pkg/startup"
	"github.com/talos-systems/talos/pkg/userdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	dataPath *string
)

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
		tls.WithCACertPEM(data.CA.Crt),
		tls.WithCertificateProvider(tlsCertProvider))
	if err != nil {
		log.Fatalf("failed to create OS-level TLS configuration: %v", err)
	}

	initClient, err := reg.NewInitServiceClient()
	if err != nil {
		log.Fatalf("init client: %v", err)
	}

	log.Println("Starting osd")
	err = factory.ListenAndServe(
		&reg.Registrator{
			Data:              data,
			InitServiceClient: initClient,
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
