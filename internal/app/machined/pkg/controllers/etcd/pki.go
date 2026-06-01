// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"
	"os"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/filetree"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// PKIController renders manifests based on templates and config/secrets.
type PKIController struct{}

// Name implements controller.Controller interface.
func (ctrl *PKIController) Name() string {
	return "etcd.PKIController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PKIController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.EtcdRootType,
			ID:        optional.Some(secrets.EtcdRootID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.EtcdType,
			ID:        optional.Some(secrets.EtcdID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *PKIController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: etcd.PKIStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *PKIController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		rootScrts, err := safe.ReaderGet[*secrets.EtcdRoot](ctx, r, resource.NewMetadata(secrets.NamespaceName, secrets.EtcdRootType, secrets.EtcdRootID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting root secrets: %w", err)
		}

		scrts, err := safe.ReaderGet[*secrets.Etcd](ctx, r, resource.NewMetadata(secrets.NamespaceName, secrets.EtcdType, secrets.EtcdID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting secrets: %w", err)
		}

		if err = os.MkdirAll(constants.EtcdPKIPath, 0o700); err != nil {
			return err
		}

		if err = selinux.SetLabel(constants.EtcdPKIPath, constants.EtcdPKISELinuxLabel); err != nil {
			return err
		}

		if err = os.WriteFile(constants.EtcdCACert, rootScrts.TypedSpec().EtcdCA.Crt, 0o400); err != nil {
			return fmt.Errorf("failed to write CA certificate: %w", err)
		}

		if err = os.WriteFile(constants.EtcdCAKey, rootScrts.TypedSpec().EtcdCA.Key, 0o400); err != nil {
			return fmt.Errorf("failed to write CA key: %w", err)
		}

		etcdCerts := scrts.TypedSpec()

		for _, keypair := range []struct {
			getter   func() *x509.PEMEncodedCertificateAndKey
			keyPath  string
			certPath string
		}{
			{
				getter:   func() *x509.PEMEncodedCertificateAndKey { return etcdCerts.Etcd },
				keyPath:  constants.EtcdKey,
				certPath: constants.EtcdCert,
			},
			{
				getter:   func() *x509.PEMEncodedCertificateAndKey { return etcdCerts.EtcdPeer },
				keyPath:  constants.EtcdPeerKey,
				certPath: constants.EtcdPeerCert,
			},
			{
				getter:   func() *x509.PEMEncodedCertificateAndKey { return etcdCerts.EtcdAdmin },
				keyPath:  constants.EtcdAdminKey,
				certPath: constants.EtcdAdminCert,
			},
		} {
			if err = os.WriteFile(keypair.keyPath, keypair.getter().Key, 0o400); err != nil {
				return err
			}

			if err = os.WriteFile(keypair.certPath, keypair.getter().Crt, 0o400); err != nil {
				return err
			}
		}

		if err = filetree.ChownRecursive(constants.EtcdPKIPath, constants.EtcdUserID, constants.EtcdUserID); err != nil {
			return err
		}

		if err = safe.WriterModify(ctx, r, etcd.NewPKIStatus(etcd.NamespaceName, etcd.PKIID), func(status *etcd.PKIStatus) error {
			status.TypedSpec().Ready = true
			status.TypedSpec().Version = scrts.Metadata().Version().String()

			return nil
		}); err != nil {
			return fmt.Errorf("error updating PKI status: %w", err)
		}

		r.ResetRestartBackoff()
	}
}
