// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"flag"
	"log"
	"runtime"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/reg"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

func init() {
	// Explicitly disable memory profiling to save around 1.4MiB of memory.
	runtime.MemProfileRate = 0

	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	flag.Parse()
}

func main() {
	log.Println("starting initial network configuration")

	config, err := configloader.NewFromStdin()
	if err != nil {
		log.Fatal(err)
	}

	nwd, err := networkd.New(config)
	if err != nil {
		log.Fatal(err)
	}

	if err = nwd.Configure(); err != nil {
		log.Fatal(err)
	}

	log.Println("completed initial network configuration")

	nwd.Renew()

	log.Fatalf("%+v", factory.ListenAndServe(
		reg.NewRegistrator(nwd),
		factory.Network("unix"),
		factory.SocketPath(constants.NetworkSocketPath),
		factory.WithDefaultLog(),
	),
	)
}
