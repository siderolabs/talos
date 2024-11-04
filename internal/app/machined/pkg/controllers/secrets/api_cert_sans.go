// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"fmt"

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

// APICertSANsController manages secrets.APICertSANs based on configuration.
type APICertSANsController struct{}

// Name implements controller.Controller interface.
func (ctrl *APICertSANsController) Name() string {
	return "secrets.APICertSANsController"
}

// Inputs implements controller.Controller interface.
//
//nolint:dupl
func (ctrl *APICertSANsController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.OSRootType,
			ID:        optional.Some(secrets.OSRootID),
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
func (ctrl *APICertSANsController) Outputs() []controller.Output {
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
func (ctrl *APICertSANsController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		apiRootRes, err := safe.ReaderGet[*secrets.OSRoot](ctx, r, resource.NewMetadata(secrets.NamespaceName, secrets.OSRootType, secrets.OSRootID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error destroying resources: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting root k8s secrets: %w", err)
		}

		apiRoot := apiRootRes.TypedSpec()

		hostnameResource, err := safe.ReaderGet[*network.HostnameStatus](ctx, r, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		hostnameStatus := hostnameResource.TypedSpec()

		addressesResource, err := safe.ReaderGet[*network.NodeAddress](ctx, r,
			resource.NewMetadata(network.NamespaceName, network.NodeAddressType, network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, k8s.NodeAddressFilterNoK8s), resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		nodeAddresses := addressesResource.TypedSpec()

		if err = safe.WriterModify(ctx, r, secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANAPIID), func(r *secrets.CertSAN) error {
			spec := r.TypedSpec()

			spec.Reset()

			spec.AppendIPs(apiRoot.CertSANIPs...)
			spec.AppendIPs(nodeAddresses.IPs()...)

			spec.AppendDNSNames(apiRoot.CertSANDNSNames...)
			spec.AppendDNSNames(hostnameStatus.Hostname, hostnameStatus.FQDN())

			spec.FQDN = hostnameStatus.FQDN()

			spec.Sort()

			return nil
		}); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *APICertSANsController) teardownAll(ctx context.Context, r controller.Runtime) error {
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
