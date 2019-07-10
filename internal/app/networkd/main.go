/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"log"

	"github.com/talos-systems/talos/internal/app/networkd/internal/reg"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/grpc/factory"
	"github.com/talos-systems/talos/internal/pkg/startup"
	"github.com/talos-systems/talos/pkg/userdata"
)

var dataPath *string

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	dataPath = flag.String("userdata", "", "the path to the user data")
	flag.Parse()
}

func main() {
	var err error

	if err = startup.RandSeed(); err != nil {
		log.Fatalf("startup: %s", err)
	}

	data, err := userdata.Open(*dataPath)
	if err != nil {
		log.Fatalf("credentials: %v", err)
	}

	api := reg.NewRegistrator(data)
	server := factory.NewServer(api)
	listener, err := factory.NewListener(
		factory.Network("unix"),
		factory.SocketPath(constants.NetworkdSocketPath),
	)
	if err != nil {
		panic(err)
	}
	defer server.Stop()

	// nolint: errcheck
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
