// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Virtual link name for external IPs.
const externalLink = "external"

// PlatformConfigApplyController applies active (or cached) platform network config to the network stack.
type PlatformConfigApplyController struct{}

// Name implements controller.Controller interface.
func (ctrl *PlatformConfigApplyController) Name() string {
	return "network.PlatformConfigApplyController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PlatformConfigApplyController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.PlatformConfigType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *PlatformConfigApplyController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.AddressSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.LinkSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.RouteSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.HostnameSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.ResolverSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.TimeServerSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.AddressStatusType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.OperatorSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.ProbeSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: runtimeres.PlatformMetadataType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *PlatformConfigApplyController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		platformConfigs, err := safe.ReaderListAll[*network.PlatformConfig](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing platform configs: %w", err)
		}

		var platformConfig *network.PlatformConfig

		// we always prefer "active" to "cached"
		for cfg := range platformConfigs.All() {
			switch cfg.Metadata().ID() {
			case network.PlatformConfigActiveID:
				platformConfig = cfg
			case network.PlatformConfigCachedID:
				if platformConfig == nil {
					platformConfig = cfg
				}
			}
		}

		// if we don't have any config yet, wait...
		if platformConfig == nil {
			continue
		}

		if err := ctrl.apply(ctx, r, platformConfig); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

//nolint:dupl,gocyclo
func (ctrl *PlatformConfigApplyController) apply(ctx context.Context, r controller.Runtime, platformConfig *network.PlatformConfig) error {
	networkConfig := platformConfig.TypedSpec()

	metadataLength := 0

	if networkConfig.Metadata != nil {
		metadataLength = 1
	}

	// handle all network specs in a loop as all specs can be handled in a similar way
	for _, specType := range []struct {
		length           int
		getter           func(i int) any
		idBuilder        func(spec any) (resource.ID, error)
		resourceBuilder  func(id string) resource.Resource
		resourceModifier func(newSpec any) func(r resource.Resource) error
	}{
		// AddressSpec
		{
			length: len(networkConfig.Addresses),
			getter: func(i int) any {
				return networkConfig.Addresses[i]
			},
			idBuilder: func(spec any) (resource.ID, error) {
				addressSpec := spec.(network.AddressSpecSpec) //nolint:forcetypeassert

				return network.LayeredID(network.ConfigPlatform, network.AddressID(addressSpec.LinkName, addressSpec.Address)), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewAddressSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec any) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.AddressSpec).TypedSpec()

					*spec = newSpec.(network.AddressSpecSpec) //nolint:forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// LinkSpec
		{
			length: len(networkConfig.Links),
			getter: func(i int) any {
				return networkConfig.Links[i]
			},
			idBuilder: func(spec any) (resource.ID, error) {
				linkSpec := spec.(network.LinkSpecSpec) //nolint:forcetypeassert

				return network.LayeredID(network.ConfigPlatform, network.LinkID(linkSpec.Name)), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewLinkSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec any) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.LinkSpec).TypedSpec()

					*spec = newSpec.(network.LinkSpecSpec) //nolint:forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// RouteSpec
		{
			length: len(networkConfig.Routes),
			getter: func(i int) any {
				return networkConfig.Routes[i]
			},
			idBuilder: func(spec any) (resource.ID, error) {
				routeSpec := spec.(network.RouteSpecSpec) //nolint:forcetypeassert

				return network.LayeredID(
					network.ConfigPlatform,
					network.RouteID(routeSpec.Table, routeSpec.Family, routeSpec.Destination, routeSpec.Gateway, routeSpec.Priority, routeSpec.OutLinkName),
				), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewRouteSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec any) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.RouteSpec).TypedSpec()

					*spec = newSpec.(network.RouteSpecSpec) //nolint:forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// HostnameSpec
		{
			length: len(networkConfig.Hostnames),
			getter: func(i int) any {
				return networkConfig.Hostnames[i]
			},
			idBuilder: func(spec any) (resource.ID, error) {
				return network.LayeredID(network.ConfigPlatform, network.HostnameID), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewHostnameSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec any) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.HostnameSpec).TypedSpec()

					*spec = newSpec.(network.HostnameSpecSpec) //nolint:forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// ResolverSpec
		{
			length: len(networkConfig.Resolvers),
			getter: func(i int) any {
				return networkConfig.Resolvers[i]
			},
			idBuilder: func(spec any) (resource.ID, error) {
				return network.LayeredID(network.ConfigPlatform, network.ResolverID), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewResolverSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec any) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.ResolverSpec).TypedSpec()

					*spec = newSpec.(network.ResolverSpecSpec) //nolint:forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// TimeServerSpec
		{
			length: len(networkConfig.TimeServers),
			getter: func(i int) any {
				return networkConfig.TimeServers[i]
			},
			idBuilder: func(spec any) (resource.ID, error) {
				return network.LayeredID(network.ConfigPlatform, network.TimeServerID), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewTimeServerSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec any) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.TimeServerSpec).TypedSpec()

					*spec = newSpec.(network.TimeServerSpecSpec) //nolint:forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// OperatorSpec
		{
			length: len(networkConfig.Operators),
			getter: func(i int) any {
				return networkConfig.Operators[i]
			},
			idBuilder: func(spec any) (resource.ID, error) {
				operatorSpec := spec.(network.OperatorSpecSpec) //nolint:forcetypeassert

				return network.LayeredID(network.ConfigPlatform, network.OperatorID(operatorSpec.Operator, operatorSpec.LinkName)), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewOperatorSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec any) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.OperatorSpec).TypedSpec()

					*spec = newSpec.(network.OperatorSpecSpec) //nolint:forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// ExternalIPs
		{
			length: len(networkConfig.ExternalIPs),
			getter: func(i int) any {
				return networkConfig.ExternalIPs[i]
			},
			idBuilder: func(spec any) (resource.ID, error) {
				ipAddr := spec.(netip.Addr) //nolint:forcetypeassert
				ipPrefix := netip.PrefixFrom(ipAddr, ipAddr.BitLen())

				return network.AddressID(externalLink, ipPrefix), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewAddressStatus(network.NamespaceName, id)
			},
			resourceModifier: func(newSpec any) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					ipAddr := newSpec.(netip.Addr) //nolint:forcetypeassert
					ipPrefix := netip.PrefixFrom(ipAddr, ipAddr.BitLen())

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
				}
			},
		},
		// ProbeSpec
		{
			length: len(networkConfig.Probes),
			getter: func(i int) any {
				return networkConfig.Probes[i]
			},
			idBuilder: func(spec any) (resource.ID, error) {
				probeSpec := spec.(network.ProbeSpecSpec) //nolint:forcetypeassert

				return probeSpec.ID()
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewProbeSpec(network.NamespaceName, id)
			},
			resourceModifier: func(newSpec any) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.ProbeSpec).TypedSpec()

					*spec = newSpec.(network.ProbeSpecSpec) //nolint:forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// Platform metadata
		{
			length: metadataLength,
			getter: func(i int) any {
				return networkConfig.Metadata
			},
			idBuilder: func(spec any) (resource.ID, error) {
				return runtimeres.PlatformMetadataID, nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return runtimeres.NewPlatformMetadataSpec(runtimeres.NamespaceName, id)
			},
			resourceModifier: func(newSpec any) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					metadata := newSpec.(*runtimeres.PlatformMetadataSpec) //nolint:forcetypeassert

					*r.(*runtimeres.PlatformMetadata).TypedSpec() = *metadata

					return nil
				}
			},
		},
	} {
		touchedIDs := make(map[resource.ID]struct{}, specType.length)

		resourceEmpty := specType.resourceBuilder("")
		resourceNamespace := resourceEmpty.Metadata().Namespace()
		resourceType := resourceEmpty.Metadata().Type()

		for i := range specType.length {
			spec := specType.getter(i)

			id, err := specType.idBuilder(spec)
			if err != nil {
				return fmt.Errorf("error building resource %s ID: %w", resourceType, err)
			}

			if err = r.Modify(ctx, specType.resourceBuilder(id), specType.resourceModifier(spec)); err != nil {
				return fmt.Errorf("error modifying resource %s: %w", resourceType, err)
			}

			touchedIDs[id] = struct{}{}
		}

		list, err := r.List(ctx, resource.NewMetadata(resourceNamespace, resourceType, "", resource.VersionUndefined))
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
				return fmt.Errorf("error deleting %s: %w", res, err)
			}
		}
	}

	return nil
}
