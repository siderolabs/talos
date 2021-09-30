// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"go.uber.org/zap"
	"inet.af/netaddr"

	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	platformerrors "github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/resources/network"
)

// Virtual link name for external IPs.
const externalLink = "external"

// PlatformConfigController manages updates hostnames and addressstatuses based on platform information.
type PlatformConfigController struct {
	V1alpha1Platform v1alpha1runtime.Platform
}

// Name implements controller.Controller interface.
func (ctrl *PlatformConfigController) Name() string {
	return "network.PlatformConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PlatformConfigController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *PlatformConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.HostnameSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.AddressStatusType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *PlatformConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	if ctrl.V1alpha1Platform == nil {
		// no platform, no work to be done
		return nil
	}

	// platform is fetched only once (but controller might fail and restart if fetching platform fails)
	hostname, err := ctrl.V1alpha1Platform.Hostname(ctx)
	if err != nil {
		if !errors.Is(err, platformerrors.ErrNoHostname) {
			return fmt.Errorf("error getting hostname: %w", err)
		}
	}

	if len(hostname) > 0 {
		id := network.LayeredID(network.ConfigPlatform, network.HostnameID)

		if err = r.Modify(
			ctx,
			network.NewHostnameSpec(network.ConfigNamespaceName, id),
			func(r resource.Resource) error {
				r.(*network.HostnameSpec).TypedSpec().ConfigLayer = network.ConfigPlatform

				return r.(*network.HostnameSpec).TypedSpec().ParseFQDN(string(hostname))
			},
		); err != nil {
			return fmt.Errorf("error modifying hostname resource: %w", err)
		}
	}

	externalIPs, err := ctrl.V1alpha1Platform.ExternalIPs(ctx)
	if err != nil {
		if !errors.Is(err, platformerrors.ErrNoExternalIPs) {
			return fmt.Errorf("error getting external IPs: %w", err)
		}
	}

	touchedIDs := make(map[resource.ID]struct{})

	for _, addr := range externalIPs {
		addr := addr

		ipAddr, _ := netaddr.FromStdIP(addr)
		ipPrefix := netaddr.IPPrefixFrom(ipAddr, ipAddr.BitLen())
		id := network.AddressID(externalLink, ipPrefix)

		if err = r.Modify(ctx, network.NewAddressStatus(network.NamespaceName, id), func(r resource.Resource) error {
			status := r.(*network.AddressStatus).TypedSpec()

			status.Address = ipPrefix
			status.LinkName = externalLink

			if ipAddr.Is4() {
				status.Family = nethelpers.FamilyInet4
			} else {
				status.Family = nethelpers.FamilyInet6
			}

			status.Scope = nethelpers.ScopeGlobal

			return nil
		}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}

		touchedIDs[id] = struct{}{}
	}

	// list resources for cleanup
	list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.AddressStatusType, "", resource.VersionUndefined))
	if err != nil {
		return fmt.Errorf("error listing resources: %w", err)
	}

	for _, res := range list.Items {
		if res.Metadata().Owner() != ctrl.Name() {
			continue
		}

		if _, ok := touchedIDs[res.Metadata().ID()]; ok {
			continue
		}

		if err = r.Destroy(ctx, res.Metadata()); err != nil {
			return fmt.Errorf("error deleting address status %s: %w", res, err)
		}
	}

	return nil
}
