// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	stdlibx509 "crypto/x509"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	timeresource "github.com/siderolabs/talos/pkg/machinery/resources/time"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// MaintenanceController manages secrets.MaintenanceServiceCerts.
type MaintenanceController struct{}

// Name implements controller.Controller interface.
func (ctrl *MaintenanceController) Name() string {
	return "secrets.MaintenanceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MaintenanceController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.MaintenanceRootType,
			ID:        optional.Some(secrets.MaintenanceRootID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.CertSANType,
			ID:        optional.Some(secrets.CertSANMaintenanceID),
			Kind:      controller.InputWeak,
		},
		// time status isn't fetched, but the fact that it is in dependencies means
		// that certs will be regenerated on time sync/jump (as reconcile will be triggered)
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      timeresource.StatusType,
			ID:        optional.Some(timeresource.StatusID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MaintenanceController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: secrets.MaintenanceServiceCertsType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *MaintenanceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	refreshTicker := time.NewTicker(x509.DefaultCertificateValidityDuration / 2)
	defer refreshTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-refreshTicker.C:
		}

		rootSecrets, err := safe.ReaderGetByID[*secrets.MaintenanceRoot](ctx, r, secrets.MaintenanceRootID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting maintenance root secrets: %w", err)
		}

		certSANs, err := safe.ReaderGetByID[*secrets.CertSAN](ctx, r, secrets.CertSANMaintenanceID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting certSANs: %w", err)
		}

		ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(rootSecrets.TypedSpec().CA)
		if err != nil {
			return fmt.Errorf("failed to parse CA certificate: %w", err)
		}

		serverCert, err := x509.NewKeyPair(ca,
			x509.IPAddresses(certSANs.TypedSpec().StdIPs()),
			x509.DNSNames(certSANs.TypedSpec().DNSNames),
			x509.CommonName(certSANs.TypedSpec().FQDN),
			x509.NotAfter(time.Now().Add(x509.DefaultCertificateValidityDuration)),
			x509.KeyUsage(stdlibx509.KeyUsageDigitalSignature),
			x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
				stdlibx509.ExtKeyUsageServerAuth,
			}),
		)
		if err != nil {
			return fmt.Errorf("failed to generate maintenance server cert: %w", err)
		}

		if err = safe.WriterModify(ctx, r, secrets.NewMaintenanceServiceCerts(),
			func(maintenanceSecrets *secrets.MaintenanceServiceCerts) error {
				spec := maintenanceSecrets.TypedSpec()

				spec.CA = &x509.PEMEncodedCertificateAndKey{
					Crt: rootSecrets.TypedSpec().CA.Crt,
				}
				spec.Server = x509.NewCertificateAndKeyFromKeyPair(serverCert)

				return nil
			}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}

		serverFingerprint, _ := x509.SPKIFingerprintFromDER(serverCert.Certificate.Certificate[0]) //nolint:errcheck

		logger.Debug("generated new certificates",
			zap.Stringer("server", serverFingerprint),
		)

		r.ResetRestartBackoff()
	}
}
