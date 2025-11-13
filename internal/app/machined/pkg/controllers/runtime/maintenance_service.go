// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-debug"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	machinedruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/maintenance"
	"github.com/siderolabs/talos/pkg/grpc/factory"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	machineryconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// MaintenanceServiceController runs the maintenance service based on the configuration.
type MaintenanceServiceController struct {
	SiderolinkPeerCheckFunc authz.SideroLinkPeerCheckFunc
	V1Alpha1Mode            machinedruntime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *MaintenanceServiceController) Name() string {
	return "runtime.MaintenanceServiceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MaintenanceServiceController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MaintenanceServiceRequestType,
			ID:        optional.Some(runtime.MaintenanceServiceRequestID),
			Kind:      controller.InputStrong,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MaintenanceServiceConfigType,
			ID:        optional.Some(runtime.MaintenanceServiceConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.MaintenanceServiceCertsType,
			ID:        optional.Some(secrets.MaintenanceServiceCertsID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MaintenanceServiceController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: config.MachineConfigType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *MaintenanceServiceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var (
		server                     *grpc.Server
		serverWg                   sync.WaitGroup
		listener                   net.Listener
		lastReachableAddresses     []string
		lastCertificateFingerprint string
		lastListenAddress          string
		usagePrinted               bool
	)

	shutdownServer := func(ctx context.Context) {
		if server != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
			defer shutdownCancel()

			factory.ServerGracefulStop(server, shutdownCtx)

			serverWg.Wait()

			server = nil
		}

		if listener != nil {
			listener.Close() //nolint:errcheck

			listener = nil
			lastReachableAddresses = nil
			lastListenAddress = ""
		}

		// clean up maintenance machine config, as we are done with the maintenance service
		err := r.Destroy(ctx, config.NewMachineConfigWithID(nil, config.MaintenanceID).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			logger.Error("failed to destroy maintenance machine config", zap.String("id", config.MaintenanceID), zap.Error(err))
		}
	}

	defer shutdownServer(context.Background())

	cfgCh := make(chan machineryconfig.Provider)
	srv := maintenance.New(cfgCh, ctrl.V1Alpha1Mode)

	injector := &authz.Injector{
		Mode:                    authz.ReadOnlyWithAdminOnSiderolink,
		SideroLinkPeerCheckFunc: ctrl.SiderolinkPeerCheckFunc,
	}

	if debug.Enabled {
		injector.Logger = logger.Sugar().Infof
	}

	tlsProvider := maintenance.NewTLSProvider()

	for {
		select {
		case <-ctx.Done():
			return nil
		case cfg := <-cfgCh:
			configResource := config.NewMachineConfigWithID(cfg, config.MaintenanceID)

			oldConfigResource, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.MaintenanceID)
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("failed to get machine config: %w", err)
			}

			if state.IsNotFoundError(err) {
				if err = r.Create(ctx, configResource); err != nil {
					return fmt.Errorf("failed to create machine config: %w", err)
				}
			} else {
				configResource.Metadata().SetVersion(oldConfigResource.Metadata().Version())

				if err = configResource.Metadata().SetOwner(oldConfigResource.Metadata().Owner()); err != nil {
					return fmt.Errorf("error setting owner: %w", err)
				}

				if err = r.Update(ctx, configResource); err != nil {
					return fmt.Errorf("failed to update machine config: %w", err)
				}
			}

			continue
		case <-r.EventCh():
		}

		request, err := safe.ReaderGetByID[*runtime.MaintenanceServiceRequest](ctx, r, runtime.MaintenanceServiceRequestID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get maintenance service request: %w", err)
		}

		if request == nil {
			// no request, nothing to do
			shutdownServer(ctx)

			continue
		}

		if request.Metadata().Phase() == resource.PhaseTearingDown {
			// stop the server & remove the finalizer
			shutdownServer(ctx)

			if err = r.RemoveFinalizer(ctx, request.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("failed to remove finalizer: %w", err)
			}

			continue
		}

		cfg, err := safe.ReaderGetByID[*runtime.MaintenanceServiceConfig](ctx, r, runtime.MaintenanceServiceConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get maintenance service config: %w", err)
		}

		cert, err := safe.ReaderGetByID[*secrets.MaintenanceServiceCerts](ctx, r, secrets.MaintenanceServiceCertsID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get maintenance service certs: %w", err)
		}

		if cert != nil {
			if err = tlsProvider.Update(cert); err != nil {
				return fmt.Errorf("failed to update tls provider: %w", err)
			}
		}

		// immediately add a finalizer
		if err = r.AddFinalizer(ctx, request.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("failed to add finalizer: %w", err)
		}

		if cfg == nil {
			// no config, nothing to do
			shutdownServer(ctx)

			continue
		}

		if listener != nil && cfg.TypedSpec().ListenAddress != lastListenAddress {
			// listen address changed, restart the server
			shutdownServer(ctx)
		}

		if listener == nil {
			listener, err = (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.TypedSpec().ListenAddress)
			if err != nil {
				return fmt.Errorf("failed to listen: %w", err)
			}

			lastListenAddress = cfg.TypedSpec().ListenAddress
		}

		if server == nil {
			tlsConfig, err := tlsProvider.TLSConfig()
			if err != nil {
				return fmt.Errorf("failed to get tls config: %w", err)
			}

			server = factory.NewServer(
				srv,
				factory.WithDefaultLog(),
				factory.ServerOptions(
					grpc.Creds(
						credentials.NewTLS(tlsConfig),
					),
				),

				factory.WithUnaryInterceptor(injector.UnaryInterceptor()),
				factory.WithStreamInterceptor(injector.StreamInterceptor()),
			)

			serverWg.Go(func() {
				//nolint:errcheck
				server.Serve(listener)
			})
		}

		// print additional information for the user on important state changes
		reachableAddresses := xslices.Map(cfg.TypedSpec().ReachableAddresses, netip.Addr.String)

		if !slices.Equal(lastReachableAddresses, reachableAddresses) {
			logger.Info("this machine is reachable at:")

			for _, addr := range reachableAddresses {
				logger.Info("\t" + addr)
			}

			lastReachableAddresses = reachableAddresses
		}

		if cert != nil {
			certificateFingerprint, err := x509.SPKIFingerprintFromPEM(cert.TypedSpec().Server.Crt)
			if err != nil {
				return fmt.Errorf("failed to get certificate fingerprint: %w", err)
			}

			fingerprint := certificateFingerprint.String()

			if fingerprint != lastCertificateFingerprint {
				logger.Info("server certificate issued", zap.String("fingerprint", fingerprint))
			}

			lastCertificateFingerprint = fingerprint
		}

		if !usagePrinted && len(reachableAddresses) > 0 && lastCertificateFingerprint != "" && !ctrl.V1Alpha1Mode.IsAgent() {
			firstIP := reachableAddresses[0]

			logger.Sugar().Info("upload configuration using talosctl:")
			logger.Sugar().Infof("\ttalosctl apply-config --insecure --nodes %s --file <config.yaml>", firstIP)
			logger.Sugar().Info("optionally with node fingerprint check:")
			logger.Sugar().Infof("\ttalosctl apply-config --insecure --nodes %s --cert-fingerprint '%s' --file <config.yaml>", firstIP, lastCertificateFingerprint)

			usagePrinted = true
		}

		r.ResetRestartBackoff()
	}
}
