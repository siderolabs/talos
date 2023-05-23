// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"fmt"
	"net/netip"
	"net/url"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
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
			ID:        pointer.To(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        pointer.To(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RootController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: secrets.EtcdRootType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: secrets.KubernetesRootType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: secrets.OSRootType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *RootController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardown(ctx, r, secrets.OSRootType, secrets.EtcdRootType, secrets.KubernetesRootType); err != nil {
					return fmt.Errorf("error destroying secrets: %w", err)
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

		if err = r.Modify(ctx, secrets.NewOSRoot(secrets.OSRootID), func(r resource.Resource) error {
			return ctrl.updateOSSecrets(cfgProvider, r.(*secrets.OSRoot).TypedSpec())
		}); err != nil {
			return err
		}

		// TODO: k8s secrets (partial) should be valid for the worker nodes as well, worker node should have machine (OS) CA cert (?)
		if machineType == machine.TypeWorker {
			if err = ctrl.teardown(ctx, r, secrets.EtcdRootType, secrets.KubernetesRootType); err != nil {
				return fmt.Errorf("error destroying secrets: %w", err)
			}

			continue
		}

		if err = r.Modify(ctx, secrets.NewEtcdRoot(secrets.EtcdRootID), func(r resource.Resource) error {
			return ctrl.updateEtcdSecrets(cfgProvider, r.(*secrets.EtcdRoot).TypedSpec())
		}); err != nil {
			return err
		}

		if err = r.Modify(ctx, secrets.NewKubernetesRoot(secrets.KubernetesRootID), func(r resource.Resource) error {
			return ctrl.updateK8sSecrets(cfgProvider, r.(*secrets.KubernetesRoot).TypedSpec())
		}); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *RootController) updateOSSecrets(cfgProvider talosconfig.Config, osSecrets *secrets.OSRootSpec) error {
	osSecrets.CA = cfgProvider.Machine().Security().CA()

	osSecrets.CertSANIPs = nil
	osSecrets.CertSANDNSNames = nil

	for _, san := range cfgProvider.Machine().Security().CertSANs() {
		if ip, err := netip.ParseAddr(san); err == nil {
			osSecrets.CertSANIPs = append(osSecrets.CertSANIPs, ip)
		} else {
			osSecrets.CertSANDNSNames = append(osSecrets.CertSANDNSNames, san)
		}
	}

	if cfgProvider.Machine().Features().KubernetesTalosAPIAccess().Enabled() {
		// add Kubernetes Talos service name to the list of SANs
		osSecrets.CertSANDNSNames = append(osSecrets.CertSANDNSNames,
			constants.KubernetesTalosAPIServiceName,
			constants.KubernetesTalosAPIServiceName+"."+constants.KubernetesTalosAPIServiceNamespace,
		)
	}

	osSecrets.Token = cfgProvider.Machine().Security().Token()

	return nil
}

func (ctrl *RootController) updateEtcdSecrets(cfgProvider talosconfig.Config, etcdSecrets *secrets.EtcdRootSpec) error {
	etcdSecrets.EtcdCA = cfgProvider.Cluster().Etcd().CA()

	if etcdSecrets.EtcdCA == nil {
		return fmt.Errorf("missing cluster.etcdCA secret")
	}

	return nil
}

func (ctrl *RootController) updateK8sSecrets(cfgProvider talosconfig.Config, k8sSecrets *secrets.KubernetesRootSpec) error {
	localEndpoint, err := url.Parse(fmt.Sprintf("https://localhost:%d", cfgProvider.Cluster().LocalAPIServerPort()))
	if err != nil {
		return err
	}

	k8sSecrets.Name = cfgProvider.Cluster().Name()
	k8sSecrets.Endpoint = cfgProvider.Cluster().Endpoint()
	k8sSecrets.LocalEndpoint = localEndpoint
	k8sSecrets.CertSANs = cfgProvider.Cluster().CertSANs()
	k8sSecrets.DNSDomain = cfgProvider.Cluster().Network().DNSDomain()

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
	k8sSecrets.SecretboxEncryptionSecret = cfgProvider.Cluster().SecretboxEncryptionSecret()

	k8sSecrets.BootstrapTokenID = cfgProvider.Cluster().Token().ID()
	k8sSecrets.BootstrapTokenSecret = cfgProvider.Cluster().Token().Secret()

	return nil
}

func (ctrl *RootController) teardown(ctx context.Context, r controller.Runtime, types ...resource.Type) error {
	// TODO: change this to proper teardown sequence
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
