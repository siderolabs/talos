// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeaccess

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/kubeaccess/serviceaccount"
	"github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubeaccess"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// CRDController manages Kubernetes endpoints resource for Talos API endpoints.
type CRDController struct{}

// Name implements controller.Controller interface.
func (ctrl *CRDController) Name() string {
	return "kubeaccess.CRDController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CRDController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      kubeaccess.ConfigType,
			ID:        optional.Some(kubeaccess.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesType,
			ID:        optional.Some(secrets.KubernetesID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.OSRootType,
			ID:        optional.Some(secrets.OSRootID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *CRDController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *CRDController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var crdControllerCtxCancel context.CancelFunc

	crdControllerErrCh := make(chan error, 1)

	stopCRDController := func() {
		if crdControllerCtxCancel != nil {
			crdControllerCtxCancel()

			<-crdControllerErrCh

			crdControllerCtxCancel = nil
		}
	}

	defer stopCRDController()

	for {
		select {
		case <-ctx.Done():
			return nil //nolint:govet
		case <-r.EventCh():
		case err := <-crdControllerErrCh:
			if crdControllerCtxCancel != nil {
				crdControllerCtxCancel()
			}

			crdControllerCtxCancel = nil

			if err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("error from crd controller: %w", err)
			}
		}

		kubeaccessConfig, err := safe.ReaderGet[*kubeaccess.Config](ctx, r, kubeaccess.NewConfig(config.NamespaceName, kubeaccess.ConfigID).Metadata())
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error fetching kubeaccess config: %w", err)
			}

			continue
		}

		var kubeaccessConfigSpec *kubeaccess.ConfigSpec

		if kubeaccessConfig != nil {
			kubeaccessConfigSpec = kubeaccessConfig.TypedSpec()
		}

		if kubeaccessConfig == nil || kubeaccessConfigSpec == nil || !kubeaccessConfigSpec.Enabled {
			stopCRDController()

			continue
		}

		kubeSecretsResources, err := safe.ReaderGet[*secrets.Kubernetes](ctx, r, resource.NewMetadata(
			secrets.NamespaceName,
			secrets.KubernetesType,
			secrets.KubernetesID,
			resource.VersionUndefined,
		))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error fetching kubernetes secrets: %w", err)
			}

			continue
		}

		kubeSecretsSpec := kubeSecretsResources.TypedSpec()

		osSecretsResource, err := safe.ReaderGet[*secrets.OSRoot](ctx, r, resource.NewMetadata(
			secrets.NamespaceName,
			secrets.OSRootType,
			secrets.OSRootID,
			resource.VersionUndefined,
		))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error fetching os secrets: %w", err)
			}

			continue
		}

		osSecretsSpec := osSecretsResource.TypedSpec()

		kubeconfig, err := clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
			return clientcmd.Load([]byte(kubeSecretsSpec.LocalhostAdminKubeconfig))
		})
		if err != nil {
			return fmt.Errorf("error loading kubeconfig: %w", err)
		}

		stopCRDController()

		var crdControllerCtx context.Context

		crdControllerCtx, crdControllerCtxCancel = context.WithCancel(ctx) //nolint:govet

		go func() {
			crdControllerErrCh <- ctrl.runCRDController(
				crdControllerCtx,
				osSecretsSpec.IssuingCA,
				kubeconfig,
				kubeaccessConfigSpec,
				logger,
			)
		}()

		r.ResetRestartBackoff()
	}
}

func (ctrl *CRDController) runCRDController(
	ctx context.Context,
	talosCA *x509.PEMEncodedCertificateAndKey,
	kubeconfig *rest.Config,
	kubeaccessCfgSpec *kubeaccess.ConfigSpec,
	logger *zap.Logger,
) error {
	return etcd.WithLock(ctx, constants.EtcdTalosServiceAccountCRDControllerMutex, logger, func() error {
		crdCtrl, err := serviceaccount.NewCRDController(
			talosCA,
			kubeconfig,
			kubeaccessCfgSpec.AllowedKubernetesNamespaces,
			kubeaccessCfgSpec.AllowedAPIRoles,
			logger,
		)
		if err != nil {
			return err
		}

		return crdCtrl.Run(ctx, 1)
	})
}
