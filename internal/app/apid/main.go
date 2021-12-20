// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package apid

import (
	"context"
	"flag"
	"log"
	"regexp"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
	debug "github.com/talos-systems/go-debug"
	"github.com/talos-systems/grpc-proxy/proxy"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	apidbackend "github.com/talos-systems/talos/internal/app/apid/pkg/backend"
	"github.com/talos-systems/talos/internal/app/apid/pkg/director"
	"github.com/talos-systems/talos/internal/app/apid/pkg/provider"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/grpc/middleware/authz"
	"github.com/talos-systems/talos/pkg/grpc/proxy/backend"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/startup"
)

var rbacEnabled *bool

func runDebugServer(ctx context.Context) {
	const debugAddr = ":9981"

	debugLogFunc := func(msg string) {
		log.Print(msg)
	}

	if err := debug.ListenAndServe(ctx, debugAddr, debugLogFunc); err != nil {
		log.Fatalf("failed to start debug server: %s", err)
	}
}

// Main is the entrypoint of apid.
func Main() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	rbacEnabled = flag.Bool("enable-rbac", false, "enable RBAC for Talos API")

	flag.Parse()

	go runDebugServer(context.TODO())

	if err := startup.RandSeed(); err != nil {
		log.Fatalf("failed to seed RNG: %v", err)
	}

	runtimeConn, err := grpc.Dial("unix://"+constants.APIRuntimeSocketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to dial runtime connection: %v", err)
	}

	stateClient := v1alpha1.NewStateClient(runtimeConn)
	resources := state.WrapCore(client.NewAdapter(stateClient))

	tlsConfig, err := provider.NewTLSConfig(resources)
	if err != nil {
		log.Fatalf("failed to create remote certificate provider: %+v", err)
	}

	serverTLSConfig, err := tlsConfig.ServerConfig()
	if err != nil {
		log.Fatalf("failed to create OS-level TLS configuration: %v", err)
	}

	clientTLSConfig, err := tlsConfig.ClientConfig()
	if err != nil {
		log.Fatalf("failed to create client TLS config: %v", err)
	}

	backendFactory := apidbackend.NewAPIDFactory(clientTLSConfig)
	localBackend := backend.NewLocal("machined", constants.MachineSocketPath)

	router := director.NewRouter(backendFactory.Get, localBackend)

	// all existing streaming methods
	for _, methodName := range []string{
		"/machine.MachineService/Copy",
		"/machine.MachineService/DiskUsage",
		"/machine.MachineService/Dmesg",
		"/machine.MachineService/EtcdSnapshot",
		"/machine.MachineService/Events",
		"/machine.MachineService/Kubeconfig",
		"/machine.MachineService/List",
		"/machine.MachineService/Logs",
		"/machine.MachineService/Read",
		"/resource.ResourceService/List",
		"/resource.ResourceService/Watch",
		"/os.OSService/Dmesg",
		"/cluster.ClusterService/HealthCheck",
	} {
		router.RegisterStreamedRegex("^" + regexp.QuoteMeta(methodName) + "$")
	}

	// register future pattern: method should have suffix "Stream"
	router.RegisterStreamedRegex("Stream$")

	var errGroup errgroup.Group

	errGroup.Go(func() error {
		mode := authz.Disabled
		if *rbacEnabled {
			mode = authz.Enabled
		}

		injector := &authz.Injector{
			Mode:   mode,
			Logger: log.New(log.Writer(), "apid/authz/injector/http ", log.Flags()).Printf,
		}

		return factory.ListenAndServe(
			router,
			factory.Port(constants.ApidPort),
			factory.WithDefaultLog(),
			factory.ServerOptions(
				grpc.Creds(
					credentials.NewTLS(serverTLSConfig),
				),
				grpc.CustomCodec(proxy.Codec()), //nolint:staticcheck
				grpc.UnknownServiceHandler(
					proxy.TransparentHandler(
						router.Director,
						proxy.WithStreamedDetector(router.StreamedDetector),
					)),
			),
			factory.WithUnaryInterceptor(injector.UnaryInterceptor()),
			factory.WithStreamInterceptor(injector.StreamInterceptor()),
		)
	})

	errGroup.Go(func() error {
		injector := &authz.Injector{
			Mode:   authz.MetadataOnly,
			Logger: log.New(log.Writer(), "apid/authz/injector/unix ", log.Flags()).Printf,
		}

		return factory.ListenAndServe(
			router,
			factory.Network("unix"),
			factory.SocketPath(constants.APISocketPath),
			factory.WithDefaultLog(),
			factory.ServerOptions(
				grpc.CustomCodec(proxy.Codec()), //nolint:staticcheck
				grpc.UnknownServiceHandler(
					proxy.TransparentHandler(
						router.Director,
						proxy.WithStreamedDetector(router.StreamedDetector),
					)),
			),
			factory.WithUnaryInterceptor(injector.UnaryInterceptor()),
			factory.WithStreamInterceptor(injector.StreamInterceptor()),
		)
	})

	if err := errGroup.Wait(); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
