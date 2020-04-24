// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"log"

	"github.com/talos-systems/talos/internal/app/osd/internal/reg"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/startup"
	"github.com/talos-systems/talos/pkg/universe"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
}

func main() {
	if err := startup.RandSeed(); err != nil {
		log.Fatalf("failed to seed RNG: %v", err)
	}

	log.Fatalf("%+v", factory.ListenAndServe(
		&reg.Registrator{},
		factory.Network("unix"),
		factory.SocketPath(universe.OSSocketPath),
		factory.WithDefaultLog(),
	),
	)
}
