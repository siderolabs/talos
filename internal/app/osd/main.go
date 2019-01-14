/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"log"

	"github.com/autonomy/talos/internal/app/osd/internal/reg"
	"github.com/autonomy/talos/internal/pkg/constants"
	"github.com/autonomy/talos/internal/pkg/grpc/factory"
	"github.com/autonomy/talos/internal/pkg/grpc/gen"
	"github.com/autonomy/talos/internal/pkg/grpc/tls"
	"github.com/autonomy/talos/internal/pkg/userdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	dataPath *string
	generate *bool
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	dataPath = flag.String("userdata", "", "the path to the user data")
	generate = flag.Bool("generate", false, "generate the TLS certificate using one of the Root of Trusts")
	flag.Parse()
}

func main() {
	data, err := userdata.Open(*dataPath)
	if err != nil {
		log.Fatalf("open user data: %v", err)
	}

	if *generate {
		var generator *gen.Generator
		generator, err = gen.NewGenerator(data, constants.TrustdPort)
		if err != nil {
			log.Fatal(err)
		}
		if err = generator.Identity(data.Security); err != nil {
			log.Fatalf("generate identity: %v", err)
		}
	}

	config, err := tls.NewConfig(tls.Mutual, data.Security.OS)
	if err != nil {
		log.Fatalf("credentials: %v", err)
	}

	log.Println("Starting osd")
	err = factory.Listen(
		&reg.Registrator{Data: data},
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
