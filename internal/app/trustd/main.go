// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package trustd implements trustd functionality.
package trustd

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
	debug "github.com/siderolabs/go-debug"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/siderolabs/talos/internal/app/trustd/internal/provider"
	"github.com/siderolabs/talos/internal/app/trustd/internal/reg"
	"github.com/siderolabs/talos/pkg/grpc/factory"
	"github.com/siderolabs/talos/pkg/grpc/middleware/auth/basic"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/startup"
)

func runDebugServer(ctx context.Context) {
	const debugAddr = ":9983"

	debugLogFunc := func(msg string) {
		log.Print(msg)
	}

	if err := debug.ListenAndServe(ctx, debugAddr, debugLogFunc); err != nil {
		log.Fatalf("failed to start debug server: %s", err)
	}
}

// Main is the entrypoint into trustd.
func Main() {
	if err := trustdMain(); err != nil {
		log.Fatal(err)
	}
}

func trustdMain() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	flag.Parse()

	go runDebugServer(ctx)

	startup.LimitMaxProcs(constants.TrustdMaxProcs)

	var err error

	runtimeConn, err := grpc.NewClient(
		"unix://"+constants.TrustdRuntimeSocketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithSharedWriteBuffer(true),
		grpc.WithNoProxy(),
	)
	if err != nil {
		return fmt.Errorf("failed to dial runtime connection: %w", err)
	}

	stateClient := v1alpha1.NewStateClient(runtimeConn)
	resources := state.WrapCore(client.NewAdapter(stateClient))

	tlsConfig, err := provider.NewTLSConfig(ctx, resources)
	if err != nil {
		return fmt.Errorf("failed to create remote certificate provider: %w", err)
	}

	serverTLSConfig, err := tlsConfig.ServerConfig()
	if err != nil {
		return fmt.Errorf("failed to create OS-level TLS configuration: %w", err)
	}

	creds := basic.NewTokenCredentialsDynamic(tokenGetter(resources))

	networkListener, err := factory.NewListener(
		factory.Port(constants.TrustdPort),
	)
	if err != nil {
		return fmt.Errorf("error creating listener: %w", err)
	}

	networkServer := factory.NewServer(
		&reg.Registrator{Resources: resources},
		factory.WithDefaultLog(),
		factory.WithUnaryInterceptor(creds.UnaryInterceptor()),
		factory.ServerOptions(
			grpc.Creds(
				credentials.NewTLS(serverTLSConfig),
			),
		),
	)

	errGroup, ctx := errgroup.WithContext(ctx)

	errGroup.Go(func() error {
		return networkServer.Serve(networkListener)
	})

	errGroup.Go(func() error {
		return tlsConfig.Watch(ctx)
	})

	errGroup.Go(func() error {
		<-ctx.Done()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		factory.ServerGracefulStop(networkServer, shutdownCtx)

		return nil
	})

	return errGroup.Wait()
}

func tokenGetter(state state.State) basic.TokenGetterFunc {
	return func(ctx context.Context) (string, error) {
		osRoot, err := safe.StateGet[*secrets.OSRoot](ctx, state, resource.NewMetadata(secrets.NamespaceName, secrets.OSRootType, secrets.OSRootID, resource.VersionUndefined))
		if err != nil {
			return "", err
		}

		return osRoot.TypedSpec().Token, nil
	}
}
