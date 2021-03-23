// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package timed

import (
	"flag"
	"log"

	"github.com/talos-systems/talos/internal/app/timed/pkg/ntp"
	"github.com/talos-systems/talos/internal/app/timed/pkg/reg"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/startup"
)

// Main is the entrypoint into timed.
//
// New instantiates a new ntp instance against a given server
// If no servers are specified, the default will be used.
func Main() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	flag.Parse()

	if err := startup.RandSeed(); err != nil {
		log.Fatalf("startup: %v", err)
	}

	config, err := configloader.NewFromStdin()
	if err != nil {
		log.Fatal(err)
	}

	// Check if ntp servers are defined
	// Support for only a single time server currently
	if len(config.Machine().Time().Servers()) == 0 {
		log.Fatal("no time servers configured")
	}

	server := config.Machine().Time().Servers()[0]

	n, err := ntp.NewNTPClient(
		ntp.WithServer(server),
	)
	if err != nil {
		log.Fatalf("failed to create ntp client: %v", err)
	}

	log.Println("starting timed")

	errch := make(chan error)

	go func() {
		errch <- n.Daemon()
	}()

	go func() {
		errch <- factory.ListenAndServe(
			reg.NewRegistrator(n),
			factory.Network("unix"),
			factory.SocketPath(constants.TimeSocketPath),
			factory.WithDefaultLog(),
		)
	}()

	log.Fatal(<-errch)
}
