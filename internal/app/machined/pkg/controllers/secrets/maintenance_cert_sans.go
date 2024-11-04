// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// MaintenanceCertSANsController manages secrets.APICertSANs based on configuration.
type MaintenanceCertSANsController struct{}

// Name implements controller.Controller interface.
func (ctrl *MaintenanceCertSANsController) Name() string {
	return "secrets.MaintenanceCertSANsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MaintenanceCertSANsController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        optional.Some(network.HostnameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        optional.Some(network.NodeAddressAccumulativeID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MaintenanceCertSANsController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: secrets.CertSANType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *MaintenanceCertSANsController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		hostnameStatus, err := safe.ReaderGetByID[*network.HostnameStatus](ctx, r, network.HostnameID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get hostname status: %w", err)
		}

		nodeAddresses, err := safe.ReaderGetByID[*network.NodeAddress](ctx, r, network.NodeAddressAccumulativeID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if err = safe.WriterModify(ctx, r, secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANMaintenanceID), func(r *secrets.CertSAN) error {
			spec := r.TypedSpec()

			spec.Reset()

			spec.AppendIPs(nodeAddresses.TypedSpec().IPs()...)
			spec.AppendIPs(netip.MustParseAddr("127.0.0.1"))
			spec.AppendIPs(netip.MustParseAddr("::1"))

			if hostnameStatus != nil {
				spec.AppendDNSNames(hostnameStatus.TypedSpec().DNSNames()...)
			}

			spec.FQDN = constants.MaintenanceServiceCommonName

			spec.Sort()

			return nil
		}); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}
