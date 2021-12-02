// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/kubespan"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	"github.com/talos-systems/talos/pkg/version"
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
			ID:        pointer.ToString(cluster.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.IdentityType,
			ID:        pointer.ToString(cluster.LocalIdentity),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        pointer.ToString(network.HostnameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodenameType,
			ID:        pointer.ToString(k8s.NodenameID),
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
			ID:        pointer.ToString(kubespan.LocalIdentity),
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
func (ctrl *LocalAffiliateController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			// mandatory resources to be fetched
			discoveryConfig, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, cluster.ConfigType, cluster.ConfigID, resource.VersionUndefined))
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting discovery config: %w", err)
				}

				continue
			}

			identity, err := r.Get(ctx, resource.NewMetadata(cluster.NamespaceName, cluster.IdentityType, cluster.LocalIdentity, resource.VersionUndefined))
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting local identity: %w", err)
				}

				continue
			}

			hostname, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined))
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting hostname: %w", err)
				}

				continue
			}

			nodename, err := r.Get(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.NodenameType, k8s.NodenameID, resource.VersionUndefined))
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting nodename: %w", err)
				}

				continue
			}

			addresses, err := r.Get(ctx,
				resource.NewMetadata(network.NamespaceName, network.NodeAddressType, network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s), resource.VersionUndefined))
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting addresses: %w", err)
				}

				continue
			}

			machineType, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting machine type: %w", err)
				}

				continue
			}

			// optional resources (kubespan)
			kubespanIdentity, err := r.Get(ctx, resource.NewMetadata(kubespan.NamespaceName, kubespan.IdentityType, kubespan.LocalIdentity, resource.VersionUndefined))
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting kubespan identity: %w", err)
			}

			ksAdditionalAddresses, err := r.Get(ctx,
				resource.NewMetadata(network.NamespaceName, network.NodeAddressType, network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterOnlyK8s), resource.VersionUndefined))
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting kubespan additional addresses: %w", err)
			}

			localID := identity.(*cluster.Identity).TypedSpec().NodeID

			touchedIDs := make(map[resource.ID]struct{})

			if discoveryConfig.(*cluster.Config).TypedSpec().DiscoveryEnabled {
				if err = r.Modify(ctx, cluster.NewAffiliate(cluster.NamespaceName, localID), func(res resource.Resource) error {
					spec := res.(*cluster.Affiliate).TypedSpec()

					spec.NodeID = localID
					spec.Addresses = append([]netaddr.IP(nil), addresses.(*network.NodeAddress).TypedSpec().IPs()...)
					spec.Hostname = hostname.(*network.HostnameStatus).TypedSpec().FQDN()
					spec.Nodename = nodename.(*k8s.Nodename).TypedSpec().Nodename
					spec.MachineType = machineType.(*config.MachineType).MachineType()
					spec.OperatingSystem = fmt.Sprintf("%s (%s)", version.Name, version.Tag)

					spec.KubeSpan = cluster.KubeSpanAffiliateSpec{}

					if kubespanIdentity != nil {
						spec.KubeSpan.Address = kubespanIdentity.(*kubespan.Identity).TypedSpec().Address.IP()
						spec.KubeSpan.PublicKey = kubespanIdentity.(*kubespan.Identity).TypedSpec().PublicKey
						spec.KubeSpan.AdditionalAddresses = append([]netaddr.IPPrefix(nil), ksAdditionalAddresses.(*network.NodeAddress).TypedSpec().Addresses...)

						nodeIPs := addresses.(*network.NodeAddress).TypedSpec().IPs()
						endpoints := make([]netaddr.IPPort, 0, len(nodeIPs))

						for i := range nodeIPs {
							if nodeIPs[i] == spec.KubeSpan.Address {
								// skip kubespan local address
								continue
							}

							endpoints = append(endpoints, netaddr.IPPortFrom(nodeIPs[i], constants.KubeSpanDefaultPort))
						}

						spec.KubeSpan.Endpoints = endpoints
					}

					return nil
				}); err != nil {
					return err
				}

				touchedIDs[localID] = struct{}{}
			}

			// list keys for cleanup
			list, err := r.List(ctx, resource.NewMetadata(cluster.NamespaceName, cluster.AffiliateType, "", resource.VersionUndefined))
			if err != nil {
				return fmt.Errorf("error listing resources: %w", err)
			}

			for _, res := range list.Items {
				if res.Metadata().Owner() != ctrl.Name() {
					continue
				}

				if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
					if err = r.Destroy(ctx, res.Metadata()); err != nil {
						return fmt.Errorf("error cleaning up specs: %w", err)
					}
				}
			}
		}
	}
}
