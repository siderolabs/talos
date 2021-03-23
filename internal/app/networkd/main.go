// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package networkd

import (
	"context"
	"io"
	"log"

	"golang.org/x/sync/errgroup"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/reg"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Main is the entrypoint into networkd.
func Main(ctx context.Context, r runtime.Runtime, logOutput io.Writer) error {
	logger := log.New(logOutput, "", log.Lshortfile|log.Ldate|log.Lmicroseconds|log.Ltime)

	defer logger.Println("networkd stopped")

	return run(ctx, r, logger)
}

func run(ctx context.Context, r runtime.Runtime, logger *log.Logger) error {
	var eg errgroup.Group

	logger.Println("starting initial network configuration")

	nwd, err := networkd.New(logger, r.Config())
	if err != nil {
		return err
	}

	if err = nwd.Configure(ctx); err != nil {
		return err
	}

	registrator, err := reg.NewRegistrator(nwd)
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
		factory.WithLog("", logger.Writer()),
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
