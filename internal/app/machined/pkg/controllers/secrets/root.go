// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"fmt"
	"log"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/secrets"
)

// RootController manages secrets.Root based on configuration.
type RootController struct{}

// Name implements controller.Controller interface.
func (ctrl *RootController) Name() string {
	return "secrets.RootController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RootController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.ToString(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        pointer.ToString(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RootController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: secrets.RootType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *RootController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error destroying static pods: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgProvider := cfg.(*config.MachineConfig).Config()

		machineTypeRes, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine type: %w", err)
		}

		machineType := machineTypeRes.(*config.MachineType).MachineType()

		if machineType != machine.TypeControlPlane && machineType != machine.TypeInit {
			if err = ctrl.teardownAll(ctx, r); err != nil {
				return fmt.Errorf("error destroying secrets: %w", err)
			}

			continue
		}

		if err = r.Modify(ctx, secrets.NewRoot(secrets.RootEtcdID), func(r resource.Resource) error {
			return ctrl.updateEtcdSecrets(cfgProvider, r.(*secrets.Root).EtcdSpec())
		}); err != nil {
			return err
		}

		if err = r.Modify(ctx, secrets.NewRoot(secrets.RootKubernetesID), func(r resource.Resource) error {
			return ctrl.updateK8sSecrets(cfgProvider, r.(*secrets.Root).KubernetesSpec())
		}); err != nil {
			return err
		}
	}
}

func (ctrl *RootController) updateEtcdSecrets(cfgProvider talosconfig.Provider, etcdSecrets *secrets.RootEtcdSpec) error {
	etcdSecrets.EtcdCA = cfgProvider.Cluster().Etcd().CA()

	if etcdSecrets.EtcdCA == nil {
		return fmt.Errorf("missing cluster.etcdCA secret")
	}

	return nil
}

func (ctrl *RootController) updateK8sSecrets(cfgProvider talosconfig.Provider, k8sSecrets *secrets.RootKubernetesSpec) error {
	k8sSecrets.Name = cfgProvider.Cluster().Name()
	k8sSecrets.Endpoint = cfgProvider.Cluster().Endpoint()
	k8sSecrets.CertSANs = cfgProvider.Cluster().CertSANs()
	k8sSecrets.DNSDomain = cfgProvider.Cluster().Network().DNSDomain()

	var err error

	k8sSecrets.APIServerIPs, err = cfgProvider.Cluster().Network().APIServerIPs()
	if err != nil {
		return fmt.Errorf("error building API service IPs: %w", err)
	}

	k8sSecrets.AggregatorCA = cfgProvider.Cluster().AggregatorCA()

	if k8sSecrets.AggregatorCA == nil {
		return fmt.Errorf("missing cluster.aggregatorCA secret")
	}

	k8sSecrets.CA = cfgProvider.Cluster().CA()

	if k8sSecrets.CA == nil {
		return fmt.Errorf("missing cluster.CA secret")
	}

	k8sSecrets.ServiceAccount = cfgProvider.Cluster().ServiceAccount()

	k8sSecrets.AESCBCEncryptionSecret = cfgProvider.Cluster().AESCBCEncryptionSecret()

	k8sSecrets.BootstrapTokenID = cfgProvider.Cluster().Token().ID()
	k8sSecrets.BootstrapTokenSecret = cfgProvider.Cluster().Token().Secret()

	return nil
}

func (ctrl *RootController) teardownAll(ctx context.Context, r controller.Runtime) error {
	list, err := r.List(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.RootType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	// TODO: change this to proper teardown sequence

	for _, res := range list.Items {
		if err = r.Destroy(ctx, res.Metadata()); err != nil {
			return err
		}
	}

	return nil
}
