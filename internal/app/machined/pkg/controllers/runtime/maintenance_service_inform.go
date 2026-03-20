// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	machinedruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// MaintenanceServiceInformController provides logging when the maintenance service is running.
type MaintenanceServiceInformController struct {
	V1Alpha1Mode machinedruntime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *MaintenanceServiceInformController) Name() string {
	return "runtime.MaintenanceServiceInformController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MaintenanceServiceInformController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MaintenanceServiceRequestType,
			ID:        optional.Some(runtime.MaintenanceServiceRequestID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MaintenanceServiceConfigType,
			ID:        optional.Some(runtime.MaintenanceServiceConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.APIType,
			ID:        optional.Some(secrets.APIID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MaintenanceServiceInformController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *MaintenanceServiceInformController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var (
		lastReachableAddresses     []string
		lastCertificateFingerprint string
		usagePrinted               bool
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		request, err := safe.ReaderGetByID[*runtime.MaintenanceServiceRequest](ctx, r, runtime.MaintenanceServiceRequestID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get maintenance service request: %w", err)
		}

		if request == nil {
			// no request, nothing to do
			continue
		}

		cfg, err := safe.ReaderGetByID[*runtime.MaintenanceServiceConfig](ctx, r, runtime.MaintenanceServiceConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get maintenance service config: %w", err)
		}

		if cfg == nil {
			// no config, nothing to do
			continue
		}

		cert, err := safe.ReaderGetByID[*secrets.API](ctx, r, secrets.APIID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get API secret: %w", err)
		}

		if cert != nil && !cert.TypedSpec().SkipVerifyingClientCert {
			// not a maintenance mode yet
			cert = nil
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
