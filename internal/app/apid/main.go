// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package apid

import (
	"context"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"reflect"
	"regexp"
	"syscall"
	"time"

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
	if err := apidMain(); err != nil {
		log.Fatal(err)
	}
}

//nolint:gocyclo
func apidMain() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	rbacEnabled := flag.Bool("enable-rbac", false, "enable RBAC for Talos API")
	extKeyUsageCheckEnabled := flag.Bool("enable-ext-key-usage-check", false, "enable check for client certificate ext key usage")

	flag.Parse()

	go runDebugServer(ctx)

	startup.LimitMaxProcs(constants.ApidMaxProcs)

	if err := startup.RandSeed(); err != nil {
		return fmt.Errorf("failed to seed RNG: %w", err)
	}

	runtimeConn, err := grpc.Dial("unix://"+constants.APIRuntimeSocketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to dial runtime connection: %w", err)
	}

	stateClient := v1alpha1.NewStateClient(runtimeConn)
	resources := state.WrapCore(client.NewAdapter(stateClient))

	tlsConfig, err := provider.NewTLSConfig(resources)
	if err != nil {
		return fmt.Errorf("failed to create remote certificate provider: %w", err)
	}

	serverTLSConfig, err := tlsConfig.ServerConfig()
	if err != nil {
		return fmt.Errorf("failed to create OS-level TLS configuration: %w", err)
	}

	if *extKeyUsageCheckEnabled {
		serverTLSConfig.VerifyPeerCertificate = verifyExtKeyUsage
	}

	clientTLSConfig, err := tlsConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to create client TLS config: %w", err)
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
		"/machine.MachineService/PacketCapture",
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

	networkListener, err := factory.NewListener(
		factory.Port(constants.ApidPort),
	)
	if err != nil {
		return fmt.Errorf("error creating listner: %w", err)
	}

	socketListener, err := factory.NewListener(
		factory.Network("unix"),
		factory.SocketPath(constants.APISocketPath),
	)
	if err != nil {
		return fmt.Errorf("error creating listner: %w", err)
	}

	networkServer := func() *grpc.Server {
		mode := authz.Disabled
		if *rbacEnabled {
			mode = authz.Enabled
		}

		injector := &authz.Injector{
			Mode:   mode,
			Logger: log.New(log.Writer(), "apid/authz/injector/http ", log.Flags()).Printf,
		}

		return factory.NewServer(
			router,
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
	}()

	socketServer := func() *grpc.Server {
		injector := &authz.Injector{
			Mode:   authz.MetadataOnly,
			Logger: log.New(log.Writer(), "apid/authz/injector/unix ", log.Flags()).Printf,
		}

		return factory.NewServer(
			router,
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
	}()

	errGroup, ctx := errgroup.WithContext(ctx)

	errGroup.Go(func() error {
		return networkServer.Serve(networkListener)
	})

	errGroup.Go(func() error {
		return socketServer.Serve(socketListener)
	})

	errGroup.Go(func() error {
		<-ctx.Done()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		factory.ServerGracefulStop(networkServer, shutdownCtx)
		factory.ServerGracefulStop(socketServer, shutdownCtx)

		return nil
	})

	return errGroup.Wait()
}

func verifyExtKeyUsage(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	if len(verifiedChains) == 0 {
		return fmt.Errorf("no verified chains")
	}

	certs := verifiedChains[0]

	for _, cert := range certs {
		if cert.IsCA {
			continue
		}

		if !reflect.DeepEqual(cert.ExtKeyUsage, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}) {
			return fmt.Errorf("certificate %q is missing the client auth extended key usage", cert.Subject)
		}
	}

	return nil
}
