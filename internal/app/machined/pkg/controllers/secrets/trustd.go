// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	stdlibx509 "crypto/x509"
	"errors"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	timeresource "github.com/siderolabs/talos/pkg/machinery/resources/time"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// TrustdController manages secrets.API based on configuration to provide apid certificate.
type TrustdController struct{}

// Name implements controller.Controller interface.
func (ctrl *TrustdController) Name() string {
	return "secrets.TrustdController"
}

// Inputs implements controller.Controller interface.
func (ctrl *TrustdController) Inputs() []controller.Input {
	// initial set of inputs: wait for machine type to be known and network to be partially configured
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.StatusType,
			ID:        optional.Some(network.StatusID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        optional.Some(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *TrustdController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: secrets.TrustdType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *TrustdController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// reset inputs back to what they were initially
		if err := r.UpdateInputs(ctrl.Inputs()); err != nil {
			return err
		}

		machineTypeRes, err := safe.ReaderGet[*config.MachineType](ctx, r, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine type: %w", err)
		}

		machineType := machineTypeRes.MachineType()

		networkResource, err := safe.ReaderGet[*network.Status](ctx, r, resource.NewMetadata(network.NamespaceName, network.StatusType, network.StatusID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		networkStatus := networkResource.TypedSpec()

		if !(networkStatus.AddressReady && networkStatus.HostnameReady) {
			continue
		}

		// machine type is known and network is ready, we can now proceed to one or another reconcile loop
		if machineType.IsControlPlane() {
			if err = ctrl.reconcile(ctx, r, logger); err != nil {
				return err
			}
		} else {
			if err = ctrl.teardownAll(ctx, r); err != nil {
				return err
			}
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocyclo,dupl
func (ctrl *TrustdController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	inputs := []controller.Input{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.OSRootType,
			ID:        optional.Some(secrets.OSRootID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.CertSANType,
			ID:        optional.Some(secrets.CertSANAPIID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        optional.Some(config.MachineTypeID),
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

	if err := r.UpdateInputs(inputs); err != nil {
		return fmt.Errorf("error updating inputs: %w", err)
	}

	r.QueueReconcile()

	refreshTicker := time.NewTicker(x509.DefaultCertificateValidityDuration / 2)
	defer refreshTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-refreshTicker.C:
		}

		machineTypeRes, err := safe.ReaderGet[*config.MachineType](ctx, r, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine type: %w", err)
		}

		machineType := machineTypeRes.MachineType()

		if !machineType.IsControlPlane() {
			return errors.New("machine type changed")
		}

		rootResource, err := safe.ReaderGet[*secrets.OSRoot](ctx, r, resource.NewMetadata(secrets.NamespaceName, secrets.OSRootType, secrets.OSRootID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error destroying resources: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting etcd root secrets: %w", err)
		}

		rootSpec := rootResource.TypedSpec()

		certSANResource, err := safe.ReaderGet[*secrets.CertSAN](ctx, r, resource.NewMetadata(secrets.NamespaceName, secrets.CertSANType, secrets.CertSANAPIID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting certSANs: %w", err)
		}

		certSANs := certSANResource.TypedSpec()

		if err := ctrl.generateControlPlane(ctx, r, logger, rootSpec, certSANs); err != nil {
			return err
		}
	}
}

func (ctrl *TrustdController) generateControlPlane(ctx context.Context, r controller.Runtime, logger *zap.Logger, rootSpec *secrets.OSRootSpec, certSANs *secrets.CertSANSpec) error {
	ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(rootSpec.IssuingCA)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	serverCert, err := x509.NewKeyPair(ca,
		x509.IPAddresses(certSANs.StdIPs()),
		x509.DNSNames(certSANs.DNSNames),
		x509.CommonName(certSANs.FQDN),
		x509.NotAfter(time.Now().Add(x509.DefaultCertificateValidityDuration)),
		x509.KeyUsage(stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment),
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageServerAuth,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to generate API server cert: %w", err)
	}

	if err := safe.WriterModify(ctx, r, secrets.NewTrustd(),
		func(r *secrets.Trustd) error {
			trustdSecrets := r.TypedSpec()

			trustdSecrets.AcceptedCAs = rootSpec.AcceptedCAs
			trustdSecrets.Server = x509.NewCertificateAndKeyFromKeyPair(serverCert)

			return nil
		}); err != nil {
		return fmt.Errorf("error modifying resource: %w", err)
	}

	serverFingerprint, _ := x509.SPKIFingerprintFromDER(serverCert.Certificate.Certificate[0]) //nolint:errcheck

	logger.Debug("generated new certificates",
		zap.Stringer("server", serverFingerprint),
	)

	return nil
}

func (ctrl *TrustdController) teardownAll(ctx context.Context, r controller.Runtime) error {
	list, err := r.List(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.TrustdType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range list.Items {
		if err = r.Destroy(ctx, res.Metadata()); err != nil {
			return err
		}
	}

	return nil
}
