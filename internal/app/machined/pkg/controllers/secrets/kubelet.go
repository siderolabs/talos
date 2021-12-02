// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
)

// KubeletController manages secrets.Kubelet based on configuration.
type KubeletController struct{}

// Name implements controller.Controller interface.
func (ctrl *KubeletController) Name() string {
	return "secrets.KubeletController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KubeletController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.ToString(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KubeletController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: secrets.KubeletType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *KubeletController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardown(ctx, r, secrets.KubeletType); err != nil {
					return fmt.Errorf("error destroying secrets: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgProvider := cfg.(*config.MachineConfig).Config()

		if err = r.Modify(ctx, secrets.NewKubelet(secrets.KubeletID), func(r resource.Resource) error {
			return ctrl.updateKubeletSecrets(cfgProvider, r.(*secrets.Kubelet).TypedSpec())
		}); err != nil {
			return err
		}
	}
}

func (ctrl *KubeletController) updateKubeletSecrets(cfgProvider talosconfig.Provider, kubeletSecrets *secrets.KubeletSpec) error {
	kubeletSecrets.Endpoint = cfgProvider.Cluster().Endpoint()

	kubeletSecrets.CA = cfgProvider.Cluster().CA()

	if kubeletSecrets.CA == nil {
		return fmt.Errorf("missing cluster.CA secret")
	}

	kubeletSecrets.BootstrapTokenID = cfgProvider.Cluster().Token().ID()
	kubeletSecrets.BootstrapTokenSecret = cfgProvider.Cluster().Token().Secret()

	return nil
}

func (ctrl *KubeletController) teardown(ctx context.Context, r controller.Runtime, types ...resource.Type) error {
	for _, resourceType := range types {
		items, err := r.List(ctx, resource.NewMetadata(secrets.NamespaceName, resourceType, "", resource.VersionUndefined))
		if err != nil {
			return err
		}

		for _, item := range items.Items {
			if err := r.Destroy(ctx, item.Metadata()); err != nil {
				return err
			}
		}
	}

	return nil
}
