/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"log"

	"github.com/talos-systems/talos/internal/app/trustd/internal/reg"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/grpc/factory"
	"github.com/talos-systems/talos/internal/pkg/grpc/middleware/auth/basic"
	"github.com/talos-systems/talos/internal/pkg/grpc/tls"
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
	var err error

	data, err := userdata.Open(*dataPath)
	if err != nil {
		log.Fatalf("credentials: %v", err)
	}

	config, err := tls.NewConfig(tls.ServerOnly, data.Security.OS)
	if err != nil {
		log.Fatalf("credentials: %v", err)
	}

	creds := basic.NewCredentials(
		data.Services.Trustd.Username,
		data.Services.Trustd.Password,
	)

	err = factory.Listen(
		&reg.Registrator{Data: data.Security.OS},
		factory.Port(constants.TrustdPort),
		factory.ServerOptions(
			grpc.Creds(
				credentials.NewTLS(config),
			),
			grpc.UnaryInterceptor(creds.UnaryInterceptor),
		),
	)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
}
