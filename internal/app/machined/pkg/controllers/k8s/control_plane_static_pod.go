// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	k8sadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/k8stemplates"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// ControlPlaneStaticPodController manages k8s.StaticPod based on control plane configuration.
type ControlPlaneStaticPodController struct{}

// Name implements controller.Controller interface.
func (ctrl *ControlPlaneStaticPodController) Name() string {
	return "k8s.ControlPlaneStaticPodController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ControlPlaneStaticPodController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.APIServerConfigType,
			ID:        optional.Some(k8s.FinalAPIServerConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.ControllerManagerConfigType,
			ID:        optional.Some(k8s.FinalControllerManagerConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.SchedulerConfigType,
			ID:        optional.Some(k8s.FinalSchedulerConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.SecretsStatusType,
			ID:        optional.Some(k8s.StaticPodSecretsStaticPodID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.ConfigStatusType,
			ID:        optional.Some(k8s.ConfigStatusStaticPodID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("etcd"),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ControlPlaneStaticPodController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.StaticPodType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *ControlPlaneStaticPodController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// wait for etcd to be healthy as kube-apiserver is using local etcd instance
		etcdResource, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, "etcd")
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get etcd service status: %w", err)
		}

		secretsStatusResource, err := safe.ReaderGetByID[*k8s.SecretsStatus](ctx, r, k8s.StaticPodSecretsStaticPodID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get secrets status resource: %w", err)
		}

		configStatusResource, err := safe.ReaderGetByID[*k8s.ConfigStatus](ctx, r, k8s.ConfigStatusStaticPodID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get config status resource: %w", err)
		}

		r.StartTrackingOutputs()

		// pre-condition to produce static pods
		if etcdResource != nil && etcdResource.TypedSpec().Healthy && configStatusResource != nil && secretsStatusResource != nil {
			configVersion := configStatusResource.TypedSpec().Version
			secretsVersion := secretsStatusResource.TypedSpec().Version

			for _, manageFunc := range []func(context.Context, controller.Runtime, *zap.Logger, string, string) error{
				ctrl.manageAPIServer,
				ctrl.manageControllerManager,
				ctrl.manageScheduler,
			} {
				if err = manageFunc(ctx, r, logger, secretsVersion, configVersion); err != nil {
					return err
				}
			}
		}

		// clean up static pods which haven't been touched
		if err := safe.CleanupOutputs[*k8s.StaticPod](ctx, r); err != nil {
			return fmt.Errorf("failed to cleanup outputs: %w", err)
		}
	}
}

//nolint:gocyclo
func (ctrl *ControlPlaneStaticPodController) manageAPIServer(ctx context.Context, r controller.Runtime, _ *zap.Logger,
	secretsVersion, configVersion string,
) error {
	configResource, err := safe.ReaderGetByID[*k8s.APIServerConfig](ctx, r, k8s.FinalAPIServerConfigID)
	if err != nil {
		if state.IsNotFoundError(err) {
			// no config => no pod
			return nil
		}

		return fmt.Errorf("failed to get apiserver config: %w", err)
	}

	pod, err := k8stemplates.APIServerPod(configResource, secretsVersion, configVersion)
	if err != nil {
		return fmt.Errorf("error building apiserver pod: %w", err)
	}

	return safe.WriterModify(ctx, r, k8s.NewStaticPod(k8s.NamespaceName, k8s.APIServerID), func(r *k8s.StaticPod) error {
		return k8sadapter.StaticPod(r).SetPod(pod)
	})
}

func (ctrl *ControlPlaneStaticPodController) manageControllerManager(ctx context.Context, r controller.Runtime,
	_ *zap.Logger, secretsVersion, _ string,
) error {
	configResource, err := safe.ReaderGetByID[*k8s.ControllerManagerConfig](ctx, r, k8s.FinalControllerManagerConfigID)
	if err != nil {
		if state.IsNotFoundError(err) {
			// no config => no pod
			return nil
		}

		return fmt.Errorf("failed to get controller-manager config: %w", err)
	}

	if !configResource.TypedSpec().Enabled {
		return nil
	}

	pod, err := k8stemplates.ControllerManagerPod(configResource, secretsVersion)
	if err != nil {
		return fmt.Errorf("error building controller-manager pod: %w", err)
	}

	return safe.WriterModify(ctx, r, k8s.NewStaticPod(k8s.NamespaceName, k8s.ControllerManagerID), func(r *k8s.StaticPod) error {
		return k8sadapter.StaticPod(r).SetPod(pod)
	})
}

func (ctrl *ControlPlaneStaticPodController) manageScheduler(ctx context.Context, r controller.Runtime,
	_ *zap.Logger, secretsVersion, _ string,
) error {
	configResource, err := safe.ReaderGetByID[*k8s.SchedulerConfig](ctx, r, k8s.FinalSchedulerConfigID)
	if err != nil {
		if state.IsNotFoundError(err) {
			// no config => no pod
			return nil
		}

		return fmt.Errorf("failed to get scheduler config: %w", err)
	}

	cfg := configResource.TypedSpec()

	if !cfg.Enabled {
		return nil
	}

	obj, err := k8stemplates.SchedulerPod(configResource, secretsVersion)
	if err != nil {
		return fmt.Errorf("error building kube-scheduler pod: %w", err)
	}

	return safe.WriterModify(ctx, r, k8s.NewStaticPod(k8s.NamespaceName, k8s.SchedulerID), func(r *k8s.StaticPod) error {
		return k8sadapter.StaticPod(r).SetPod(obj)
	})
}
