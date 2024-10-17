// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/discovery-api/api/v1alpha1/client/pb"
	discoveryclient "github.com/siderolabs/discovery-client/pkg/client"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/httpdefaults"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

const defaultDiscoveryTTL = 30 * time.Minute

// DiscoveryServiceController pushes Affiliate resource to the Kubernetes registry.
type DiscoveryServiceController struct {
	localAffiliateID       resource.ID
	discoveryConfigVersion resource.Version
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
			Namespace: kubespan.NamespaceName,
			Type:      kubespan.EndpointType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MachineResetSignalType,
			ID:        optional.Some(runtime.MachineResetSignalID),
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
		{
			Type: network.AddressStatusType,
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
		prevOtherEndpoints []discoveryclient.Endpoint
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

		cleanupClient := func() {
			if clientCtxCancel != nil {
				clientCtxCancel()

				<-clientErrCh

				clientCtxCancel = nil
				client = nil

				prevLocalData = nil
				prevLocalEndpoints = nil
				prevOtherEndpoints = nil
			}
		}

		discoveryConfig, err := safe.ReaderGetByID[*cluster.Config](ctx, r, cluster.ConfigID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting discovery config: %w", err)
			}

			continue
		}

		if !discoveryConfig.TypedSpec().RegistryServiceEnabled {
			// if discovery is disabled cleanup existing resources
			if err = cleanupAffiliates(ctx, ctrl, r, nil); err != nil {
				return err
			}

			cleanupClient()

			continue
		}

		if !discoveryConfig.Metadata().Version().Equal(ctrl.discoveryConfigVersion) {
			// force reconnect on config change
			cleanupClient()
		}

		identity, err := safe.ReaderGetByID[*cluster.Identity](ctx, r, cluster.LocalIdentity)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting local identity: %w", err)
			}

			continue
		}

		localAffiliateID := identity.TypedSpec().NodeID

		if ctrl.localAffiliateID != localAffiliateID {
			ctrl.localAffiliateID = localAffiliateID

			if err = r.UpdateInputs(append(ctrl.Inputs(),
				controller.Input{
					Namespace: cluster.NamespaceName,
					Type:      cluster.AffiliateType,
					ID:        optional.Some(ctrl.localAffiliateID),
					Kind:      controller.InputWeak,
				},
			)); err != nil {
				return err
			}

			cleanupClient()
		}

		affiliate, err := safe.ReaderGetByID[*cluster.Affiliate](ctx, r, ctrl.localAffiliateID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting local affiliate: %w", err)
			}

			continue
		}

		affiliateSpec := affiliate.TypedSpec()

		otherEndpointsList, err := safe.ReaderListAll[*kubespan.Endpoint](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing endpoints: %w", err)
		}

		machineResetSginal, err := safe.ReaderGetByID[*runtime.MachineResetSignal](ctx, r, runtime.MachineResetSignalID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting machine reset signal: %w", err)
		}

		if client == nil {
			var cipherBlock cipher.Block

			cipherBlock, err = aes.NewCipher(discoveryConfig.TypedSpec().ServiceEncryptionKey)
			if err != nil {
				return fmt.Errorf("error initializing AES cipher: %w", err)
			}

			client, err = discoveryclient.NewClient(discoveryclient.Options{
				Cipher:        cipherBlock,
				Endpoint:      discoveryConfig.TypedSpec().ServiceEndpoint,
				ClusterID:     discoveryConfig.TypedSpec().ServiceClusterID,
				AffiliateID:   localAffiliateID,
				TTL:           defaultDiscoveryTTL,
				Insecure:      discoveryConfig.TypedSpec().ServiceEndpointInsecure,
				ClientVersion: version.Tag,
				TLSConfig: &tls.Config{
					RootCAs: httpdefaults.RootCAs(),
				},
			})
			if err != nil {
				return fmt.Errorf("error initializing discovery client: %w", err)
			}

			var clientCtx context.Context

			clientCtx, clientCtxCancel = context.WithCancel(ctx) //nolint:govet

			ctrl.discoveryConfigVersion = discoveryConfig.Metadata().Version()

			go func() {
				clientErrCh <- client.Run(clientCtx, logger, notifyCh)
			}()
		}

		// delete/update local affiliate
		//
		// if the node enters final resetting stage, cleanup the local affiliate
		// otherwise, update local affiliate data
		if machineResetSginal != nil {
			client.DeleteLocalAffiliate()
		} else {
			localData := pbAffiliate(affiliateSpec)
			localEndpoints := pbEndpoints(affiliateSpec)
			otherEndpoints := pbOtherEndpoints(otherEndpointsList)

			// don't send updates on localData if it hasn't changed: this introduces positive feedback loop,
			// as the watch loop will notify on self update
			if !proto.Equal(localData, prevLocalData) || !equalEndpoints(localEndpoints, prevLocalEndpoints) || !equalOtherEndpoints(otherEndpoints, prevOtherEndpoints) {
				if err = client.SetLocalData(&discoveryclient.Affiliate{
					Affiliate: localData,
					Endpoints: localEndpoints,
				}, otherEndpoints); err != nil {
					return fmt.Errorf("error setting local affiliate data: %w", err)
				}

				prevLocalData = localData
				prevLocalEndpoints = localEndpoints
				prevOtherEndpoints = otherEndpoints
			}
		}

		// discover public IP
		if publicIP := client.GetPublicIP(); len(publicIP) > 0 {
			if err = safe.WriterModify(ctx, r, network.NewAddressStatus(cluster.NamespaceName, "service"), func(address *network.AddressStatus) error {
				var addr netip.Addr

				if err = addr.UnmarshalBinary(publicIP); err != nil {
					return fmt.Errorf("error unmarshaling public IP: %w", err)
				}

				address.TypedSpec().Address = netip.PrefixFrom(addr, addr.BitLen())

				return nil
			}); err != nil {
				return err //nolint:govet
			}
		}

		// discover other nodes (affiliates)
		touchedIDs := make(map[resource.ID]struct{})

		for _, discoveredAffiliate := range client.GetAffiliates() {
			id := fmt.Sprintf("service/%s", discoveredAffiliate.Affiliate.NodeId)

			if err = safe.WriterModify(ctx, r, cluster.NewAffiliate(cluster.RawNamespaceName, id), func(res *cluster.Affiliate) error {
				*res.TypedSpec() = specAffiliate(discoveredAffiliate.Affiliate, discoveredAffiliate.Endpoints)

				return nil
			}); err != nil {
				return err
			}

			touchedIDs[id] = struct{}{}
		}

		if err := cleanupAffiliates(ctx, ctrl, r, touchedIDs); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func pbAffiliate(affiliate *cluster.AffiliateSpec) *pb.Affiliate {
	addresses := xslices.Map(affiliate.Addresses, func(address netip.Addr) []byte {
		return takeResult(address.MarshalBinary())
	})

	var kubeSpan *pb.KubeSpan

	if affiliate.KubeSpan.PublicKey != "" {
		kubeSpan = &pb.KubeSpan{
			PublicKey: affiliate.KubeSpan.PublicKey,
			Address:   takeResult(affiliate.KubeSpan.Address.MarshalBinary()),
			AdditionalAddresses: xslices.Map(affiliate.KubeSpan.AdditionalAddresses, func(address netip.Prefix) *pb.IPPrefix {
				return &pb.IPPrefix{
					Bits: uint32(address.Bits()),
					Ip:   takeResult(address.Addr().MarshalBinary()),
				}
			}),
		}
	}

	return &pb.Affiliate{
		NodeId:          affiliate.NodeID,
		Addresses:       addresses,
		Hostname:        affiliate.Hostname,
		Nodename:        affiliate.Nodename,
		MachineType:     affiliate.MachineType.String(),
		OperatingSystem: affiliate.OperatingSystem,
		Kubespan:        kubeSpan,
		ControlPlane:    toPlane(affiliate.ControlPlane),
	}
}

func toPlane(data *cluster.ControlPlane) *pb.ControlPlane {
	if data == nil {
		return nil
	}

	return &pb.ControlPlane{ApiServerPort: uint32(data.APIServerPort)}
}

func pbEndpoints(affiliate *cluster.AffiliateSpec) []*pb.Endpoint {
	if affiliate.KubeSpan.PublicKey == "" || len(affiliate.KubeSpan.Endpoints) == 0 {
		return nil
	}

	return xslices.Map(affiliate.KubeSpan.Endpoints, func(endpoint netip.AddrPort) *pb.Endpoint {
		return &pb.Endpoint{
			Port: uint32(endpoint.Port()),
			Ip:   takeResult(endpoint.Addr().MarshalBinary()),
		}
	})
}

func pbOtherEndpoints(otherEndpointsList safe.List[*kubespan.Endpoint]) []discoveryclient.Endpoint {
	if otherEndpointsList.Len() == 0 {
		return nil
	}

	result := make([]discoveryclient.Endpoint, 0, otherEndpointsList.Len())

	for endpoint := range otherEndpointsList.All() {
		endpointSpec := endpoint.TypedSpec()

		result = append(result, discoveryclient.Endpoint{
			AffiliateID: endpointSpec.AffiliateID,
			Endpoints: []*pb.Endpoint{
				{
					Port: uint32(endpointSpec.Endpoint.Port()),
					Ip:   takeResult(endpointSpec.Endpoint.Addr().MarshalBinary()),
				},
			},
		})
	}

	return result
}

func equalEndpoints(a, b []*pb.Endpoint) bool {
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

func equalOtherEndpoints(a, b []discoveryclient.Endpoint) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].AffiliateID != b[i].AffiliateID {
			return false
		}

		if !equalEndpoints(a[i].Endpoints, b[i].Endpoints) {
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
		MachineType:     takeResult(machine.ParseType(affiliate.MachineType)), // ignore parse error (machine.TypeUnknown)
		ControlPlane:    fromControlPlane(affiliate.ControlPlane),
	}

	result.Addresses = make([]netip.Addr, 0, len(affiliate.Addresses))

	for i := range affiliate.Addresses {
		var ip netip.Addr

		if err := ip.UnmarshalBinary(affiliate.Addresses[i]); err == nil {
			result.Addresses = append(result.Addresses, ip)
		}
	}

	if affiliate.Kubespan != nil {
		result.KubeSpan.PublicKey = affiliate.Kubespan.PublicKey
		result.KubeSpan.Address.UnmarshalBinary(affiliate.Kubespan.Address) //nolint:errcheck // ignore error, address will be zero

		result.KubeSpan.AdditionalAddresses = make([]netip.Prefix, 0, len(affiliate.Kubespan.AdditionalAddresses))

		for i := range affiliate.Kubespan.AdditionalAddresses {
			var ip netip.Addr

			if err := ip.UnmarshalBinary(affiliate.Kubespan.AdditionalAddresses[i].Ip); err == nil {
				result.KubeSpan.AdditionalAddresses = append(result.KubeSpan.AdditionalAddresses, netip.PrefixFrom(ip, int(affiliate.Kubespan.AdditionalAddresses[i].Bits)))
			}
		}

		result.KubeSpan.Endpoints = make([]netip.AddrPort, 0, len(endpoints))

		for i := range endpoints {
			var ip netip.Addr

			if err := ip.UnmarshalBinary(endpoints[i].Ip); err == nil {
				result.KubeSpan.Endpoints = append(result.KubeSpan.Endpoints, netip.AddrPortFrom(ip, uint16(endpoints[i].Port)))
			}
		}
	}

	return result
}

func fromControlPlane(plane *pb.ControlPlane) *cluster.ControlPlane {
	if plane == nil {
		return nil
	}

	return &cluster.ControlPlane{APIServerPort: int(plane.ApiServerPort)}
}

func takeResult[T any](arg1 T, _ error) T {
	return arg1
}
