// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package apid

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"regexp"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-debug"
	"github.com/siderolabs/grpc-proxy/proxy"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	apidbackend "github.com/siderolabs/talos/internal/app/apid/pkg/backend"
	"github.com/siderolabs/talos/internal/app/apid/pkg/director"
	"github.com/siderolabs/talos/internal/app/apid/pkg/provider"
	"github.com/siderolabs/talos/pkg/grpc/factory"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/grpc/proxy/backend"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func runService(ctx context.Context, resources state.State, config *runtime.APIServiceConfig) error {
	log.Printf(
		"starting apid with config: listen address %s, skip client cert verify %v, node routing disabled %v, readonly role mode %v",
		config.TypedSpec().ListenAddress,
		config.TypedSpec().SkipVerifyingClientCert,
		config.TypedSpec().NodeRoutingDisabled,
		config.TypedSpec().ReadonlyRoleMode,
	)

	tlsConfig, err := provider.NewTLSConfig(ctx, resources, config.TypedSpec().SkipVerifyingClientCert)
	if err != nil {
		return fmt.Errorf("failed to create remote certificate provider: %w", err)
	}

	serverTLSConfig, err := tlsConfig.ServerConfig()
	if err != nil {
		return fmt.Errorf("failed to create OS-level TLS configuration: %w", err)
	}

	serverTLSConfig.VerifyPeerCertificate = verifyExtKeyUsage

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

		defer backendFactory.Flush()
	}

	localAddressProvider, err := director.NewLocalAddressProvider(resources)
	if err != nil {
		return fmt.Errorf("failed to create local address provider: %w", err)
	}

	localBackend := backend.NewLocal("machined", constants.MachineSocketPath)
	defer localBackend.Close() //nolint:errcheck

	router := director.NewRouter(remoteFactory, localBackend, localAddressProvider, config.TypedSpec().NodeRoutingDisabled)

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
		"/machine.MachineService/DebugContainer",
		"/machine.MachineService/Logs",
		"/machine.MachineService/PacketCapture",
		"/machine.MachineService/Read",
		"/machine.LifecycleService/Install",
		"/machine.LifecycleService/Upgrade",
		"/os.OSService/Dmesg",
		"/cluster.ClusterService/HealthCheck",
	} {
		router.RegisterStreamedRegex("^" + regexp.QuoteMeta(methodName) + "$")
	}

	// register future pattern: method should have suffix "Stream"
	router.RegisterStreamedRegex("Stream$")

	networkListener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", config.TypedSpec().ListenAddress)
	if err != nil {
		return fmt.Errorf("error creating listner: %w", err)
	}

	networkServer := func() *grpc.Server {
		injector := &authz.Injector{
			Mode: authz.Enabled,
		}

		if config.TypedSpec().ReadonlyRoleMode {
			injector.Mode = authz.ReadOnlyWithAdminOnSiderolink
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

	errGroup, ctx := errgroup.WithContext(ctx)

	errGroup.Go(func() error {
		return networkServer.Serve(networkListener)
	})

	errGroup.Go(func() error {
		return tlsConfig.Watch(ctx, onPKIUpdate)
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
