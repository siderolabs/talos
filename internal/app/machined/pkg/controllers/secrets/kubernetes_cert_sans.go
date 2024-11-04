// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// KubernetesCertSANsController manages secrets.KubernetesCertSANs based on configuration.
type KubernetesCertSANsController struct{}

// Name implements controller.Controller interface.
func (ctrl *KubernetesCertSANsController) Name() string {
	return "secrets.KubernetesCertSANsController"
}

// Inputs implements controller.Controller interface.
//
//nolint:dupl
func (ctrl *KubernetesCertSANsController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesRootType,
			ID:        optional.Some(secrets.KubernetesRootID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        optional.Some(network.HostnameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        optional.Some(network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, k8s.NodeAddressFilterNoK8s)),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KubernetesCertSANsController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: secrets.CertSANType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *KubernetesCertSANsController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		k8sRootRes, err := safe.ReaderGet[*secrets.KubernetesRoot](ctx, r, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesRootType, secrets.KubernetesRootID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error destroying resources: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting root k8s secrets: %w", err)
		}

		k8sRoot := k8sRootRes.TypedSpec()

		hostnameResource, err := safe.ReaderGet[*network.HostnameStatus](ctx, r, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		hostnameStatus := hostnameResource.TypedSpec()

		addressesResource, err := safe.ReaderGet[*network.NodeAddress](ctx,
			r,
			resource.NewMetadata(network.NamespaceName, network.NodeAddressType, network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, k8s.NodeAddressFilterNoK8s), resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		nodeAddresses := addressesResource.TypedSpec()

		if err = safe.WriterModify(ctx, r, secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANKubernetesID), func(r *secrets.CertSAN) error {
			spec := r.TypedSpec()

			spec.Reset()

			spec.Append(k8sRoot.Endpoint.Hostname())
			spec.Append(k8sRoot.CertSANs...)

			spec.AppendDNSNames(
				"kubernetes",
				"kubernetes.default",
				"kubernetes.default.svc",
				"kubernetes.default.svc."+k8sRoot.DNSDomain,
				"localhost",
			)

			spec.Append(
				hostnameStatus.Hostname,
				hostnameStatus.FQDN(),
			)

			spec.AppendIPs(k8sRoot.APIServerIPs...)
			spec.AppendIPs(nodeAddresses.IPs()...)
			spec.AppendIPs(netip.MustParseAddr("127.0.0.1"))

			spec.Sort()

			return nil
		}); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *KubernetesCertSANsController) teardownAll(ctx context.Context, r controller.Runtime) error {
	list, err := r.List(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.CertSANType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range list.Items {
		if res.Metadata().Owner() == ctrl.Name() {
			if err = r.Destroy(ctx, res.Metadata()); err != nil {
				return err
			}
		}
	}

	return nil
}
