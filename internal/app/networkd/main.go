// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"golang.org/x/sync/errgroup"

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
	if err := run(); err != nil {
		log.Fatal(err)
	}

	log.Println("networkd stopped")
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, os.Interrupt)

		select {
		case <-sigCh:
		case <-ctx.Done():
		}
	}()

	var eg errgroup.Group

	log.Println("starting initial network configuration")

	config, err := configloader.NewFromStdin()
	if err != nil {
		return err
	}

	nwd, err := networkd.New(config)
	if err != nil {
		return err
	}

	if err = nwd.Configure(ctx); err != nil {
		return err
	}

	if err = nwd.RunControllers(ctx, &eg); err != nil {
		return err
	}

	log.Println("completed initial network configuration")

	nwd.Renew(ctx)

	server := factory.NewServer(
		reg.NewRegistrator(nwd),
		factory.WithDefaultLog(),
	)

	listener, err := factory.NewListener(
		factory.Network("unix"),
		factory.SocketPath(constants.NetworkSocketPath),
	)
	if err != nil {
		return err
	}

	eg.Go(func() error {
		return server.Serve(listener)
	})

	<-ctx.Done()

	server.GracefulStop()

	return eg.Wait()
}
