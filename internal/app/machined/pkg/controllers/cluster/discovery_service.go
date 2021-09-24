// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/discovery-service/api/v1alpha1/client/pb"
	discoveryclient "github.com/talos-systems/discovery-service/pkg/client"
	"go.uber.org/zap"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/proto"
	"github.com/talos-systems/talos/pkg/resources/cluster"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/version"
)

const defaultDiscoveryTTL = 30 * time.Minute

// DiscoveryServiceController pushes Affiliate resource to the Kubernetes registry.
type DiscoveryServiceController struct {
	Insecure bool // only for testing

	localAffiliateID resource.ID
}

// Name implements controller.Controller interface.
func (ctrl *DiscoveryServiceController) Name() string {
	return "cluster.DiscoveryServiceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DiscoveryServiceController) Inputs() []controller.Input {
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
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *DiscoveryServiceController) Outputs() []controller.Output {
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
func (ctrl *DiscoveryServiceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var (
		client          *discoveryclient.Client
		clientCtxCancel context.CancelFunc
	)

	clientErrCh := make(chan error, 1)

	defer func() {
		if clientCtxCancel != nil {
			clientCtxCancel()

			<-clientErrCh
		}
	}()

	notifyCh := make(chan struct{}, 1)

	var (
		prevLocalData      *pb.Affiliate
		prevLocalEndpoints []*pb.Endpoint
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-notifyCh:
		case err := <-clientErrCh:
			if clientCtxCancel != nil {
				clientCtxCancel()
			}

			clientCtxCancel = nil

			if err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("error from discovery client: %w", err)
			}
		}

		discoveryConfig, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, cluster.ConfigType, cluster.ConfigID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting discovery config: %w", err)
			}

			continue
		}

		if !discoveryConfig.(*cluster.Config).TypedSpec().RegistryServiceEnabled {
			if clientCtxCancel != nil {
				clientCtxCancel()

				<-clientErrCh

				clientCtxCancel = nil
				client = nil

				prevLocalData = nil
				prevLocalEndpoints = nil
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

		localAffiliateID := identity.(*cluster.Identity).TypedSpec().NodeID

		if ctrl.localAffiliateID != localAffiliateID {
			ctrl.localAffiliateID = localAffiliateID

			if err = r.UpdateInputs(append(ctrl.Inputs(),
				controller.Input{
					Namespace: cluster.NamespaceName,
					Type:      cluster.AffiliateType,
					ID:        pointer.ToString(ctrl.localAffiliateID),
					Kind:      controller.InputWeak,
				},
			)); err != nil {
				return err
			}

			if clientCtxCancel != nil {
				clientCtxCancel()

				<-clientErrCh

				clientCtxCancel = nil
				client = nil

				prevLocalData = nil
				prevLocalEndpoints = nil
			}
		}

		affiliate, err := r.Get(ctx, resource.NewMetadata(cluster.NamespaceName, cluster.AffiliateType, ctrl.localAffiliateID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting local affiliate: %w", err)
			}

			continue
		}

		affiliateSpec := affiliate.(*cluster.Affiliate).TypedSpec()

		if client == nil {
			var cipher cipher.Block

			cipher, err = aes.NewCipher(discoveryConfig.(*cluster.Config).TypedSpec().ServiceEncryptionKey)
			if err != nil {
				return fmt.Errorf("error initializing AES cipher: %w", err)
			}

			client, err = discoveryclient.NewClient(discoveryclient.Options{
				Cipher:        cipher,
				Endpoint:      discoveryConfig.(*cluster.Config).TypedSpec().ServiceEndpoint,
				ClusterID:     discoveryConfig.(*cluster.Config).TypedSpec().ServiceClusterID,
				AffiliateID:   localAffiliateID,
				TTL:           defaultDiscoveryTTL,
				Insecure:      ctrl.Insecure,
				ClientVersion: version.Tag,
			})
			if err != nil {
				return fmt.Errorf("error initializing discovery client: %w", err)
			}

			var clientCtx context.Context

			clientCtx, clientCtxCancel = context.WithCancel(ctx) //nolint:govet

			go func() {
				clientErrCh <- client.Run(clientCtx, logger, notifyCh)
			}()
		}

		localData := pbAffiliate(affiliateSpec)
		localEndpoints := pbEndpoints(affiliateSpec)

		// don't send updates on localData if it hasn't changed: this introduces positive feedback loop,
		// as the watch loop will notify on self update
		if !proto.Equal(localData, prevLocalData) || !equalEndpoints(localEndpoints, prevLocalEndpoints) {
			if err = client.SetLocalData(&discoveryclient.Affiliate{
				Affiliate: localData,
				Endpoints: localEndpoints,
			}, nil); err != nil {
				return fmt.Errorf("error setting local affiliate data: %w", err) //nolint:govet
			}

			prevLocalData = localData
			prevLocalEndpoints = localEndpoints
		}

		touchedIDs := make(map[resource.ID]struct{})

		for _, discoveredAffiliate := range client.GetAffiliates() {
			id := fmt.Sprintf("service/%s", discoveredAffiliate.Affiliate.NodeId)

			discoveredAffiliate := discoveredAffiliate

			if err = r.Modify(ctx, cluster.NewAffiliate(cluster.RawNamespaceName, id), func(res resource.Resource) error {
				*res.(*cluster.Affiliate).TypedSpec() = specAffiliate(discoveredAffiliate.Affiliate, discoveredAffiliate.Endpoints)

				return nil
			}); err != nil {
				return err
			}

			touchedIDs[id] = struct{}{}
		}

		// list keys for cleanup
		list, err := r.List(ctx, resource.NewMetadata(cluster.RawNamespaceName, cluster.AffiliateType, "", resource.VersionUndefined))
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

func pbAffiliate(affiliate *cluster.AffiliateSpec) *pb.Affiliate {
	addresses := make([][]byte, len(affiliate.Addresses))

	for i := range addresses {
		addresses[i], _ = affiliate.Addresses[i].MarshalBinary() //nolint:errcheck // doesn't fail
	}

	var kubeSpan *pb.KubeSpan

	if affiliate.KubeSpan.PublicKey != "" {
		kubeSpan = &pb.KubeSpan{
			PublicKey: affiliate.KubeSpan.PublicKey,
		}

		kubeSpan.Address, _ = affiliate.KubeSpan.Address.MarshalBinary() //nolint:errcheck // doesn't fail

		additionalAddresses := make([]*pb.IPPrefix, len(affiliate.KubeSpan.AdditionalAddresses))

		for i := range additionalAddresses {
			additionalAddresses[i] = &pb.IPPrefix{
				Bits: uint32(affiliate.KubeSpan.AdditionalAddresses[i].Bits()),
			}

			additionalAddresses[i].Ip, _ = affiliate.KubeSpan.AdditionalAddresses[i].IP().MarshalBinary() //nolint:errcheck // doesn't fail
		}

		kubeSpan.AdditionalAddresses = additionalAddresses
	}

	return &pb.Affiliate{
		NodeId:          affiliate.NodeID,
		Addresses:       addresses,
		Hostname:        affiliate.Hostname,
		Nodename:        affiliate.Nodename,
		MachineType:     affiliate.MachineType.String(),
		OperatingSystem: affiliate.OperatingSystem,
		Kubespan:        kubeSpan,
	}
}

func pbEndpoints(affiliate *cluster.AffiliateSpec) []*pb.Endpoint {
	if affiliate.KubeSpan.PublicKey == "" || len(affiliate.KubeSpan.Endpoints) == 0 {
		return nil
	}

	result := make([]*pb.Endpoint, len(affiliate.KubeSpan.Endpoints))

	for i := range result {
		result[i] = &pb.Endpoint{
			Port: uint32(affiliate.KubeSpan.Endpoints[i].Port()),
		}

		result[i].Ip, _ = affiliate.KubeSpan.Endpoints[i].IP().MarshalBinary() //nolint:errcheck // doesn't fail
	}

	return result
}

func equalEndpoints(a []*pb.Endpoint, b []*pb.Endpoint) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !proto.Equal(a[i], b[i]) {
			return false
		}
	}

	return true
}

func specAffiliate(affiliate *pb.Affiliate, endpoints []*pb.Endpoint) cluster.AffiliateSpec {
	result := cluster.AffiliateSpec{
		NodeID:          affiliate.NodeId,
		Hostname:        affiliate.Hostname,
		Nodename:        affiliate.Nodename,
		OperatingSystem: affiliate.OperatingSystem,
	}

	result.MachineType, _ = machine.ParseType(affiliate.MachineType) //nolint:errcheck // ignore parse error (machine.TypeUnknown)

	result.Addresses = make([]netaddr.IP, 0, len(affiliate.Addresses))

	for i := range affiliate.Addresses {
		var ip netaddr.IP

		if err := ip.UnmarshalBinary(affiliate.Addresses[i]); err == nil {
			result.Addresses = append(result.Addresses, ip)
		}
	}

	if affiliate.Kubespan != nil {
		result.KubeSpan.PublicKey = affiliate.Kubespan.PublicKey
		result.KubeSpan.Address.UnmarshalBinary(affiliate.Kubespan.Address) //nolint:errcheck // ignore error, address will be zero

		result.KubeSpan.AdditionalAddresses = make([]netaddr.IPPrefix, 0, len(affiliate.Kubespan.AdditionalAddresses))

		for i := range affiliate.Kubespan.AdditionalAddresses {
			var ip netaddr.IP

			if err := ip.UnmarshalBinary(affiliate.Kubespan.AdditionalAddresses[i].Ip); err == nil {
				result.KubeSpan.AdditionalAddresses = append(result.KubeSpan.AdditionalAddresses, netaddr.IPPrefixFrom(ip, uint8(affiliate.Kubespan.AdditionalAddresses[i].Bits)))
			}
		}

		result.KubeSpan.Endpoints = make([]netaddr.IPPort, 0, len(endpoints))

		for i := range endpoints {
			var ip netaddr.IP

			if err := ip.UnmarshalBinary(endpoints[i].Ip); err == nil {
				result.KubeSpan.Endpoints = append(result.KubeSpan.Endpoints, netaddr.IPPortFrom(ip, uint16(endpoints[i].Port)))
			}
		}
	}

	return result
}
