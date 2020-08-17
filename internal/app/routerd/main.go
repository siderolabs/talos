// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"log"

	"google.golang.org/grpc"

	"github.com/talos-systems/grpc-proxy/proxy"

	"github.com/talos-systems/talos/internal/app/routerd/pkg/director"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/grpc/proxy/backend"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/startup"
)

func main() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	if err := startup.RandSeed(); err != nil {
		log.Fatalf("failed to seed RNG: %v", err)
	}

	router := director.NewRouter()

	// TODO: this should be dynamic based on plugin registration
	machinedBackend := backend.NewLocal("machined", constants.MachineSocketPath)
	router.RegisterLocalBackend("os.OSService", machinedBackend)
	router.RegisterLocalBackend("machine.MachineService", machinedBackend)
	router.RegisterLocalBackend("time.TimeService", backend.NewLocal("timed", constants.TimeSocketPath))
	router.RegisterLocalBackend("network.NetworkService", backend.NewLocal("networkd", constants.NetworkSocketPath))
	router.RegisterLocalBackend("cluster.ClusterService", machinedBackend)

	err := factory.ListenAndServe(
		router,
		factory.Network("unix"),
		factory.SocketPath(constants.RouterdSocketPath),
		factory.WithDefaultLog(),
		factory.ServerOptions(
			grpc.CustomCodec(proxy.Codec()),
			grpc.UnknownServiceHandler(
				proxy.TransparentHandler(
					router.Director,
				)),
		),
	)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
}
