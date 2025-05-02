// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package apid implements apid functionality.
package apid

import (
	"context"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"regexp"
	"slices"
	"syscall"
	"time"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/protobuf/client"
	"github.com/siderolabs/go-debug"
	"github.com/siderolabs/grpc-proxy/proxy"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	apidbackend "github.com/siderolabs/talos/internal/app/apid/pkg/backend"
	"github.com/siderolabs/talos/internal/app/apid/pkg/director"
	"github.com/siderolabs/talos/internal/app/apid/pkg/provider"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/grpc/factory"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/grpc/proxy/backend"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/startup"
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

	runtimeConn, err := grpc.NewClient("unix://"+constants.APIRuntimeSocketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
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

	if *extKeyUsageCheckEnabled {
		serverTLSConfig.VerifyPeerCertificate = verifyExtKeyUsage
	}

	clientTLSConfig, err := tlsConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to create client TLS config: %w", err)
	}

	var (
		remoteFactory director.RemoteBackendFactory
		onPKIUpdate   func()
	)

	if clientTLSConfig != nil {
		backendFactory := apidbackend.NewAPIDFactory(tlsConfig)
		remoteFactory = backendFactory.Get
		onPKIUpdate = backendFactory.Flush
	}

	localAddressProvider, err := director.NewLocalAddressProvider(resources)
	if err != nil {
		return fmt.Errorf("failed to create local address provider: %w", err)
	}

	localBackend := backend.NewLocal("machined", constants.MachineSocketPath)

	router := director.NewRouter(remoteFactory, localBackend, localAddressProvider)

	// all existing streaming methods
	for _, methodName := range []string{
		"/machine.MachineService/Copy",
		"/machine.MachineService/DiskUsage",
		"/machine.MachineService/Dmesg",
		"/machine.MachineService/EtcdSnapshot",
		"/machine.MachineService/Events",
		"/machine.MachineService/ImageList",
		"/machine.MachineService/Kubeconfig",
		"/machine.MachineService/List",
		"/machine.MachineService/Logs",
		"/machine.MachineService/PacketCapture",
		"/machine.MachineService/Read",
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

	if err = selinux.SetLabel(constants.APISocketPath, constants.APISocketLabel); err != nil {
		return err
	}

	networkServer := func() *grpc.Server {
		mode := authz.Disabled
		if *rbacEnabled {
			mode = authz.Enabled
		}

		injector := &authz.Injector{
			Mode: mode,
		}

		if debug.Enabled {
			injector.Logger = log.New(log.Writer(), "apid/authz/injector/http ", log.Flags()).Printf
		}

		return factory.NewServer(
			router,
			factory.WithDefaultLog(),
			factory.ServerOptions(
				grpc.Creds(
					credentials.NewTLS(serverTLSConfig),
				),
				grpc.ForceServerCodecV2(proxy.Codec()),
				grpc.UnknownServiceHandler(
					proxy.TransparentHandler(
						router.Director,
						proxy.WithStreamedDetector(router.StreamedDetector),
					),
				),
				grpc.MaxRecvMsgSize(constants.GRPCMaxMessageSize),
			),
			factory.WithUnaryInterceptor(injector.UnaryInterceptor()),
			factory.WithStreamInterceptor(injector.StreamInterceptor()),
		)
	}()

	socketServer := func() *grpc.Server {
		injector := &authz.Injector{
			Mode: authz.MetadataOnly,
		}

		if debug.Enabled {
			injector.Logger = log.New(log.Writer(), "apid/authz/injector/unix ", log.Flags()).Printf
		}

		return factory.NewServer(
			router,
			factory.WithDefaultLog(),
			factory.ServerOptions(
				grpc.ForceServerCodecV2(proxy.Codec()),
				grpc.UnknownServiceHandler(
					proxy.TransparentHandler(
						router.Director,
						proxy.WithStreamedDetector(router.StreamedDetector),
					),
				),
				grpc.MaxRecvMsgSize(constants.GRPCMaxMessageSize),
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
		return tlsConfig.Watch(ctx, onPKIUpdate)
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

func verifyExtKeyUsage(_ [][]byte, verifiedChains [][]*x509.Certificate) error {
	if len(verifiedChains) == 0 {
		return errors.New("no verified chains")
	}

	certs := verifiedChains[0]

	for _, cert := range certs {
		if cert.IsCA {
			continue
		}

		if !slices.Equal(cert.ExtKeyUsage, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}) {
			return fmt.Errorf("certificate %q is missing the client auth extended key usage", cert.Subject)
		}
	}

	return nil
}
