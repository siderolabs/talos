/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/talos-systems/talos/internal/app/osd/internal/reg"
	bully "github.com/talos-systems/talos/internal/pkg/bully/server"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/grpc/factory"
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
	data, err := userdata.Open(*dataPath)
	if err != nil {
		log.Fatalf("open user data: %v", err)
	}

	config, err := tls.NewConfig(tls.Mutual, data.Security.OS)
	if err != nil {
		log.Fatalf("credentials: %v", err)
	}

	initClient, err := reg.NewInitServiceClient()
	if err != nil {
		log.Fatalf("init client: %v", err)
	}

	registrator := &reg.Registrator{
		Data:              data,
		InitServiceClient: initClient,
	}

	if data.IsMaster() {
		bully := bully.NewBullyServer(uint32(0), "", data.Services.Trustd.Endpoints...)
		if err = bully.Join(); err != nil {
			log.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err = bully.Elect(ctx, &empty.Empty{}); err != nil {
			log.Fatal(err)
		}
	}

	log.Println("Starting osd")

	err = factory.ListenAndServe(
		registrator,
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
