// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package apid implements apid functionality.
package apid

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
	"github.com/siderolabs/gen/panicsafe"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/startup"
)

// Main is the entrypoint of apid.
func Main() {
	if err := apidMain(); err != nil {
		log.Fatal(err)
	}
}

// apidMain is the entrypoint of apid.
//
// It fetches service config as a resource and keeps watching it
// for changes.
//
// If the service config changes, it shuts down the listener and starts a new one with the new configuration.
//
//nolint:gocyclo
func apidMain() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	flag.Parse()

	go runDebugServer(ctx)

	startup.LimitMaxProcs(constants.ApidMaxProcs)

	runtimeConn, err := grpc.NewClient(
		"unix://"+constants.APIRuntimeSocketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithNoProxy(),
	)
	if err != nil {
		return fmt.Errorf("failed to dial runtime connection: %w", err)
	}

	stateClient := v1alpha1.NewStateClient(runtimeConn)
	resources := state.WrapCore(client.NewAdapter(stateClient))

	configWatchCh := make(chan safe.WrappedStateEvent[*runtime.APIServiceConfig])

	if err = safe.StateWatch(ctx, resources, runtime.NewAPIServiceConfig().Metadata(), configWatchCh); err != nil {
		return fmt.Errorf("failed to set up watch for API service config: %w", err)
	}

	var (
		serviceConfig *runtime.APIServiceConfig
		cancelService context.CancelFunc
	)

	serviceErrCh := make(chan error, 1)

outerLoop:
	for {
		select {
		case <-ctx.Done():
			break outerLoop
		case err = <-serviceErrCh:
			cancelService = nil

			if err != nil {
				return fmt.Errorf("service error: %w", err)
			}
		case event := <-configWatchCh:
			switch event.Type() {
			case state.Created, state.Updated:
				serviceConfig, err = event.Resource()
				if err != nil {
					return fmt.Errorf("failed to get API service config from watch event: %w", err) //nolint:govet
				}
			case state.Destroyed:
				serviceConfig = nil
			case state.Errored:
				return fmt.Errorf("service config watch error: %w", event.Error())
			case state.Bootstrapped, state.Noop:
				// ignore
				continue outerLoop
			}
		}

		// we got a change in the service config, restart the server with the new config
		if cancelService != nil {
			cancelService()

			cancelService = nil

			// wait for the service to shut down
			<-serviceErrCh
		}

		if serviceConfig != nil {
			var serviceCtx context.Context

			serviceCtx, cancelService = context.WithCancel(ctx) //nolint:govet

			go func() {
				serviceErrCh <- panicsafe.RunErr(func() error {
					return runService(serviceCtx, resources, serviceConfig)
				})
			}()
		}
	}

	if cancelService != nil {
		cancelService()

		<-serviceErrCh
	}

	return nil
}
