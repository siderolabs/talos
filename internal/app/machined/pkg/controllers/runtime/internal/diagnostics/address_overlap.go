// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package diagnostics

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// AddressOverlapCheck checks for overlapping host and Kubernetes pod/service CIDR addresses.
func AddressOverlapCheck(ctx context.Context, r controller.Reader, logger *zap.Logger) (*runtime.DiagnosticSpec, error) {
	hostAddresses, err := safe.ReaderGetByID[*network.NodeAddress](ctx, r, network.NodeAddressRoutedID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error reading host addresses: %w", err)
	}

	hostMinusK8s, err := safe.ReaderGetByID[*network.NodeAddress](ctx, r, network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s))
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error reading host minus k8s addresses: %w", err)
	}

	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error reading machine configuration: %w", err)
	}

	if len(hostAddresses.TypedSpec().Addresses) > 0 && len(hostMinusK8s.TypedSpec().Addresses) == 0 {
		details := []string{
			fmt.Sprintf("host routed addresses: %q", xslices.Map(hostAddresses.TypedSpec().Addresses, netip.Prefix.String)),
		}

		if cfg.Config().Cluster() != nil {
			details = append(details, fmt.Sprintf("Kubernetes pod CIDRs: %q", cfg.Config().Cluster().Network().PodCIDRs()))
			details = append(details, fmt.Sprintf("Kubernetes service CIDRs: %q", cfg.Config().Cluster().Network().ServiceCIDRs()))
		}

		return &runtime.DiagnosticSpec{
			Message: "host and Kubernetes pod/service CIDR addresses overlap",
			Details: details,
		}, nil
	}

	return nil, nil
}
