// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
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
}

func main() {
	logger := log.New(os.Stderr, "", log.Lshortfile|log.Ldate|log.Lmicroseconds|log.Ltime)

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

	if err := run(ctx, logger); err != nil {
		logger.Fatal(err)
	}

	logger.Println("networkd stopped")
}

func run(ctx context.Context, logger *log.Logger) error {
	var eg errgroup.Group

	config, err := configloader.NewFromStdin()
	if err != nil {
		return err
	}

	logger.Println("starting initial network configuration")

	nwd, err := networkd.New(logger, config)
	if err != nil {
		return err
	}

	if err = nwd.Configure(ctx); err != nil {
		return err
	}

	registrator, err := reg.NewRegistrator(logger, nwd)
	if err != nil {
		return err
	}

	if err = nwd.RunControllers(ctx, &eg); err != nil {
		return err
	}

	logger.Println("completed initial network configuration")

	nwd.Renew(ctx)

	server := factory.NewServer(
		registrator,
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
