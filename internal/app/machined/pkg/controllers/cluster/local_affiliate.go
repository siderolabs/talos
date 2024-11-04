// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/net"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// LocalAffiliateController builds Affiliate resource for the local node.
type LocalAffiliateController struct{}

// Name implements controller.Controller interface.
func (ctrl *LocalAffiliateController) Name() string {
	return "cluster.LocalAffiliateController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LocalAffiliateController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      cluster.ConfigType,
			ID:        optional.Some(cluster.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.IdentityType,
			ID:        optional.Some(cluster.LocalIdentity),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        optional.Some(network.HostnameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodenameType,
			ID:        optional.Some(k8s.NodenameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: kubespan.NamespaceName,
			Type:      kubespan.IdentityType,
			ID:        optional.Some(kubespan.LocalIdentity),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      kubespan.ConfigType,
			ID:        optional.Some(kubespan.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        optional.Some(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cluster.NamespaceName,
			Type:      network.AddressStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.APIServerConfigType,
			ID:        optional.Some(k8s.APIServerConfigID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LocalAffiliateController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cluster.AffiliateType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *LocalAffiliateController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// mandatory resources to be fetched
		discoveryConfig, err := safe.ReaderGetByID[*cluster.Config](ctx, r, cluster.ConfigID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting discovery config: %w", err)
			}

			continue
		}

		identity, err := safe.ReaderGetByID[*cluster.Identity](ctx, r, cluster.LocalIdentity)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting local identity: %w", err)
			}

			continue
		}

		hostname, err := safe.ReaderGetByID[*network.HostnameStatus](ctx, r, network.HostnameID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting hostname: %w", err)
			}

			continue
		}

		nodename, err := safe.ReaderGetByID[*k8s.Nodename](ctx, r, k8s.NodenameID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting nodename: %w", err)
			}

			continue
		}

		routedAddresses, err := safe.ReaderGetByID[*network.NodeAddress](ctx, r, network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting addresses: %w", err)
			}

			continue
		}

		currentAddresses, err := safe.ReaderGetByID[*network.NodeAddress](ctx, r, network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting addresses: %w", err)
			}

			continue
		}

		machineType, err := safe.ReaderGetByID[*config.MachineType](ctx, r, config.MachineTypeID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting machine type: %w", err)
			}

			continue
		}

		// optional resources (kubespan)
		kubespanIdentity, err := safe.ReaderGetByID[*kubespan.Identity](ctx, r, kubespan.LocalIdentity)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting kubespan identity: %w", err)
		}

		kubespanConfig, err := safe.ReaderGetByID[*kubespan.Config](ctx, r, kubespan.ConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting kubespan config: %w", err)
		}

		ksAdditionalAddresses, err := safe.ReaderGetByID[*network.NodeAddress](ctx, r, network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterOnlyK8s))
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting kubespan additional addresses: %w", err)
		}

		discoveredPublicIPs, err := safe.ReaderList[*network.AddressStatus](ctx, r, resource.NewMetadata(cluster.NamespaceName, network.AddressStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error getting discovered public IP: %w", err)
		}

		// optional resources (kubernetes)
		apiServerConfig, err := safe.ReaderGetByID[*k8s.APIServerConfig](ctx, r, k8s.APIServerConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting API server config: %w", err)
		}

		localID := identity.TypedSpec().NodeID

		touchedIDs := map[resource.ID]struct{}{}

		if discoveryConfig.TypedSpec().DiscoveryEnabled {
			if err = safe.WriterModify(ctx, r, cluster.NewAffiliate(cluster.NamespaceName, localID), func(res *cluster.Affiliate) error {
				spec := res.TypedSpec()

				spec.NodeID = localID
				spec.Hostname = hostname.TypedSpec().FQDN()
				spec.Nodename = nodename.TypedSpec().Nodename
				spec.MachineType = machineType.MachineType()
				spec.OperatingSystem = fmt.Sprintf("%s (%s)", version.Name, version.Tag)

				if machineType.MachineType().IsControlPlane() && apiServerConfig != nil {
					spec.ControlPlane = &cluster.ControlPlane{
						APIServerPort: apiServerConfig.TypedSpec().LocalPort,
					}
				} else {
					spec.ControlPlane = nil
				}

				routedNodeIPs := routedAddresses.TypedSpec().IPs()
				currentNodeIPs := currentAddresses.TypedSpec().IPs()

				spec.Addresses = routedNodeIPs

				spec.KubeSpan = cluster.KubeSpanAffiliateSpec{}

				if kubespanIdentity != nil && kubespanConfig != nil {
					spec.KubeSpan.Address = kubespanIdentity.TypedSpec().Address.Addr()
					spec.KubeSpan.PublicKey = kubespanIdentity.TypedSpec().PublicKey

					if kubespanConfig.TypedSpec().AdvertiseKubernetesNetworks && ksAdditionalAddresses != nil {
						spec.KubeSpan.AdditionalAddresses = slices.Clone(ksAdditionalAddresses.TypedSpec().Addresses)
					} else {
						spec.KubeSpan.AdditionalAddresses = nil
					}

					endpointIPs := xslices.Filter(currentNodeIPs, func(ip netip.Addr) bool {
						if ip == spec.KubeSpan.Address {
							// skip kubespan local address
							return false
						}

						if network.IsULA(ip, network.ULASideroLink) {
							// ignore SideroLink addresses, as they are point-to-point addresses
							return false
						}

						return true
					})

					// mix in discovered public IPs
					for res := range discoveredPublicIPs.All() {
						addr := res.TypedSpec().Address.Addr()

						if slices.ContainsFunc(endpointIPs, func(a netip.Addr) bool { return addr == a }) {
							// this address is already published
							continue
						}

						endpointIPs = append(endpointIPs, addr)
					}

					// filter endpoints if configured
					if kubespanConfig.TypedSpec().EndpointFilters != nil {
						endpointIPs, err = net.FilterIPs(endpointIPs, kubespanConfig.TypedSpec().EndpointFilters)
						if err != nil {
							return fmt.Errorf("error filtering KubeSpan endpoints: %w", err)
						}
					}

					spec.KubeSpan.Endpoints = xslices.Map(endpointIPs, func(addr netip.Addr) netip.AddrPort {
						return netip.AddrPortFrom(addr, constants.KubeSpanDefaultPort)
					})

					// add extra announced endpoints, deduplicating on the way
					for _, addr := range kubespanConfig.TypedSpec().ExtraEndpoints {
						if !slices.Contains(spec.KubeSpan.Endpoints, addr) {
							spec.KubeSpan.Endpoints = append(spec.KubeSpan.Endpoints, addr)
						}
					}
				}

				return nil
			}); err != nil {
				return err
			}

			touchedIDs[localID] = struct{}{}
		}

		// list keys for cleanup
		list, err := safe.ReaderListAll[*cluster.Affiliate](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for res := range list.All() {
			if res.Metadata().Owner() != ctrl.Name() {
				continue
			}

			if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up specs: %w", err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}
