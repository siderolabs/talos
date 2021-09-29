// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/kubespan"
	"github.com/talos-systems/talos/pkg/resources/network"
)

// DefaultPeerReconcileInterval is interval between peer status reconciliation on timer.
//
// Peers might be reconciled more often e.g. when peerSpecs are updated.
const DefaultPeerReconcileInterval = 30 * time.Second

// ManagerController sets up Wireguard networking based on KubeSpan configuration, watches and updates peer statuses.
type ManagerController struct {
	WireguardClientFactory WireguardClientFactory
	RulesManagerFactory    RulesManagerFactory
	NfTablesManagerFactory NfTablesManagerFactory
	PeerReconcileInterval  time.Duration
}

// Name implements controller.Controller interface.
func (ctrl *ManagerController) Name() string {
	return "kubespan.ManagerController"
}

// WireguardClientFactory allows mocking Wireguard client.
type WireguardClientFactory func() (WireguardClient, error)

// WireguardClient allows mocking Wireguard client.
type WireguardClient interface {
	Device(string) (*wgtypes.Device, error)
	Close() error
}

// RulesManagerFactory allows mocking RulesManager.
type RulesManagerFactory func(targetTable, internalMark int) RulesManager

// NfTablesManagerFactory allows mocking NfTablesManager.
type NfTablesManagerFactory func(externalMark, internalMark uint32) NfTablesManager

// Inputs implements controller.Controller interface.
func (ctrl *ManagerController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      kubespan.ConfigType,
			ID:        pointer.ToString(kubespan.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: kubespan.NamespaceName,
			Type:      kubespan.PeerSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: kubespan.NamespaceName,
			Type:      kubespan.IdentityType,
			ID:        pointer.ToString(kubespan.LocalIdentity),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ManagerController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.LinkSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.AddressSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.RouteSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: kubespan.PeerStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *ManagerController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var (
		tickerC <-chan time.Time
		ticker  *time.Ticker
	)

	if ctrl.WireguardClientFactory == nil {
		ctrl.WireguardClientFactory = func() (WireguardClient, error) {
			return wgctrl.New()
		}
	}

	if ctrl.RulesManagerFactory == nil {
		ctrl.RulesManagerFactory = NewRulesManager
	}

	if ctrl.NfTablesManagerFactory == nil {
		ctrl.NfTablesManagerFactory = NewNfTablesManager
	}

	if ctrl.PeerReconcileInterval == 0 {
		ctrl.PeerReconcileInterval = DefaultPeerReconcileInterval
	}

	wgClient, err := ctrl.WireguardClientFactory()
	if err != nil {
		return fmt.Errorf("error creating wireguard client: %w", err)
	}

	defer wgClient.Close() //nolint:errcheck

	var rulesMgr RulesManager

	defer func() {
		if rulesMgr != nil {
			if err := rulesMgr.Cleanup(); err != nil {
				logger.Error("failed cleaning up routing rules", zap.Error(err))
			}
		}
	}()

	var nfTablesMgr NfTablesManager

	defer func() {
		if nfTablesMgr != nil {
			if err := nfTablesMgr.Cleanup(); err != nil {
				logger.Error("failed cleaning up nftables rules", zap.Error(err))
			}
		}
	}()

	for {
		var updateSpecs bool

		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			updateSpecs = true
		case <-tickerC:
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, kubespan.ConfigType, kubespan.ConfigID, resource.VersionUndefined))
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting kubespan configuration: %w", err)
		}

		if cfg == nil || !cfg.(*kubespan.Config).TypedSpec().Enabled {
			if ticker != nil {
				ticker.Stop()

				tickerC = nil
			}

			// KubeSpan is not enabled, cleanup everything
			if err = ctrl.cleanup(ctx, r); err != nil {
				return err
			}

			if rulesMgr != nil {
				if err = rulesMgr.Cleanup(); err != nil {
					logger.Error("failed cleaning up routing rules", zap.Error(err))
				}

				rulesMgr = nil
			}

			if nfTablesMgr != nil {
				if err = nfTablesMgr.Cleanup(); err != nil {
					logger.Error("failed cleaning up nftables rules", zap.Error(err))
				}

				nfTablesMgr = nil
			}

			continue
		}

		if ticker == nil {
			ticker = time.NewTicker(ctrl.PeerReconcileInterval)
			tickerC = ticker.C
		}

		if rulesMgr == nil {
			rulesMgr = ctrl.RulesManagerFactory(constants.KubeSpanDefaultRoutingTable, constants.KubeSpanDefaultForceFirewallMark)

			if err = rulesMgr.Install(); err != nil {
				return fmt.Errorf("failed setting up routing rules: %w", err)
			}
		}

		if nfTablesMgr == nil {
			nfTablesMgr = ctrl.NfTablesManagerFactory(constants.KubeSpanDefaultFirewallMark, constants.KubeSpanDefaultForceFirewallMark)
		}

		cfgSpec := cfg.(*kubespan.Config).TypedSpec()

		localIdentity, err := r.Get(ctx, resource.NewMetadata(kubespan.NamespaceName, kubespan.IdentityType, kubespan.LocalIdentity, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting local KubeSpan identity: %w", err)
		}

		localSpec := localIdentity.(*kubespan.Identity).TypedSpec()

		// fetch PeerSpecs and PeerStatuses and sync them
		peerSpecList, err := r.List(ctx, resource.NewMetadata(kubespan.NamespaceName, kubespan.PeerSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing peer specs: %w", err)
		}

		peerSpecs := make(map[string]*kubespan.PeerSpecSpec, len(peerSpecList.Items))

		for _, res := range peerSpecList.Items {
			peerSpecs[res.Metadata().ID()] = res.(*kubespan.PeerSpec).TypedSpec()
		}

		peerStatusList, err := r.List(ctx, resource.NewMetadata(kubespan.NamespaceName, kubespan.PeerStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing peer status: %w", err)
		}

		peerStatuses := make(map[string]*kubespan.PeerStatusSpec, len(peerStatusList.Items))

		for _, res := range peerStatusList.Items {
			// drop any peer statuses which are not in the peer specs
			if _, ok := peerSpecs[res.Metadata().ID()]; !ok {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error destroying peer status: %w", err)
				}

				continue
			}

			peerStatuses[res.Metadata().ID()] = res.(*kubespan.PeerStatus).TypedSpec()
		}

		// create missing peer statuses
		for pubKey, peerSpec := range peerSpecs {
			if _, ok := peerStatuses[pubKey]; !ok {
				peerStatuses[pubKey] = &kubespan.PeerStatusSpec{
					Label: peerSpec.Label,
				}
			}
		}

		// update peer status from Wireguard data
		wgDevice, err := wgClient.Device(constants.KubeSpanLinkName)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("error fetching wireguard link status: %w", err)
		}

		if wgDevice != nil { // wgDevice might be nil if the link is not created yet
			for _, peerInfo := range wgDevice.Peers {
				if peerStatus, ok := peerStatuses[peerInfo.PublicKey.String()]; ok {
					peerStatus.UpdateFromWireguard(peerInfo)
				}
			}
		}

		// calculate peer status connection state
		for _, peerStatus := range peerStatuses {
			peerStatus.CalculateState()
		}

		// build wireguard peer configuration
		wgPeers := make([]network.WireguardPeer, 0, len(peerSpecs))

		for pubKey, peerSpec := range peerSpecs {
			// list of statuses and specs should be in sync at this point
			peerStatus := peerStatuses[pubKey]

			var endpoint string

			// check if the endpoint should be updated
			if peerStatus.ShouldChangeEndpoint() {
				newEndpoint := peerStatus.PickNewEndpoint(peerSpec.Endpoints)

				if !newEndpoint.IsZero() {
					logger.Debug("updating endpoint for the peer", zap.String("peer", pubKey), zap.String("label", peerSpec.Label), zap.Stringer("endpoint", newEndpoint))

					endpoint = newEndpoint.String()
					peerStatus.UpdateEndpoint(newEndpoint)

					updateSpecs = true
				}
			}

			// re-establish the endpoint if it wasn't applied to the Wireguard config completely
			if !peerStatus.LastUsedEndpoint.IsZero() && (peerStatus.Endpoint.IsZero() || peerStatus.Endpoint == peerStatus.LastUsedEndpoint) {
				endpoint = peerStatus.LastUsedEndpoint.String()
				peerStatus.Endpoint = peerStatus.LastUsedEndpoint

				updateSpecs = true
			}

			wgPeers = append(wgPeers, network.WireguardPeer{
				PublicKey:                   pubKey,
				PresharedKey:                cfgSpec.SharedSecret,
				Endpoint:                    endpoint,
				PersistentKeepaliveInterval: constants.KubeSpanDefaultPeerKeepalive,
				AllowedIPs:                  append([]netaddr.IPPrefix(nil), peerSpec.AllowedIPs...),
			})
		}

		// build full allowedIPs set
		var allowedIPsBuilder netaddr.IPSetBuilder

		for pubKey, peerSpec := range peerSpecs {
			// list of statuses and specs should be in sync at this point
			peerStatus := peerStatuses[pubKey]

			// add allowedIPs to the nftables set if either routing is forced (for any peer state)
			// or if the peer connection state is up.
			if cfgSpec.ForceRouting || peerStatus.State == kubespan.PeerStateUp {
				for _, prefix := range peerSpec.AllowedIPs {
					allowedIPsBuilder.AddPrefix(prefix)
				}
			}
		}

		allowedIPsSet, err := allowedIPsBuilder.IPSet()
		if err != nil {
			return fmt.Errorf("failed building allowed IPs set: %w", err)
		}

		// update peer statuses
		for pubKey, peerStatus := range peerStatuses {
			peerStatus := peerStatus

			if err = r.Modify(ctx,
				kubespan.NewPeerStatus(
					kubespan.NamespaceName,
					pubKey,
				),
				func(r resource.Resource) error {
					*r.(*kubespan.PeerStatus).TypedSpec() = *peerStatus

					return nil
				},
			); err != nil {
				return fmt.Errorf("error modifying peer status: %w", err)
			}
		}

		if !updateSpecs {
			// micro-optimization: skip updating specs if there are no changes to the incoming resources and no endpoint changes
			continue
		}

		if err = r.Modify(ctx,
			network.NewAddressSpec(
				network.ConfigNamespaceName,
				network.LayeredID(network.ConfigOperator, network.AddressID(constants.KubeSpanLinkName, localSpec.Address)),
			),
			func(r resource.Resource) error {
				spec := r.(*network.AddressSpec).TypedSpec()

				spec.Address = netaddr.IPPrefixFrom(localSpec.Address.IP(), localSpec.Subnet.Bits())
				spec.ConfigLayer = network.ConfigOperator
				spec.Family = nethelpers.FamilyInet6
				spec.Flags = nethelpers.AddressFlags(nethelpers.AddressPermanent)
				spec.LinkName = constants.KubeSpanLinkName
				spec.Scope = nethelpers.ScopeGlobal

				return nil
			},
		); err != nil {
			return fmt.Errorf("error modifying address: %w", err)
		}

		for _, spec := range []network.RouteSpecSpec{
			{
				Family:      nethelpers.FamilyInet4,
				Destination: netaddr.IPPrefix{},
				Source:      netaddr.IP{},
				Gateway:     netaddr.IP{},
				OutLinkName: constants.KubeSpanLinkName,
				Table:       nethelpers.RoutingTable(constants.KubeSpanDefaultRoutingTable),
				Priority:    1,
				Scope:       nethelpers.ScopeGlobal,
				Type:        nethelpers.TypeUnicast,
				Flags:       0,
				Protocol:    nethelpers.ProtocolStatic,
				ConfigLayer: network.ConfigOperator,
			},
			{
				Family:      nethelpers.FamilyInet6,
				Destination: netaddr.IPPrefix{},
				Source:      netaddr.IP{},
				Gateway:     netaddr.IP{},
				OutLinkName: constants.KubeSpanLinkName,
				Table:       nethelpers.RoutingTable(constants.KubeSpanDefaultRoutingTable),
				Priority:    1,
				Scope:       nethelpers.ScopeGlobal,
				Type:        nethelpers.TypeUnicast,
				Flags:       0,
				Protocol:    nethelpers.ProtocolStatic,
				ConfigLayer: network.ConfigOperator,
			},
		} {
			spec := spec

			if err = r.Modify(ctx,
				network.NewRouteSpec(
					network.ConfigNamespaceName,
					network.LayeredID(network.ConfigOperator, network.RouteID(spec.Table, spec.Family, spec.Destination, spec.Gateway, spec.Priority)),
				),
				func(r resource.Resource) error {
					*r.(*network.RouteSpec).TypedSpec() = spec

					return nil
				},
			); err != nil {
				return fmt.Errorf("error modifying route spec: %w", err)
			}
		}

		if err = r.Modify(ctx,
			network.NewLinkSpec(
				network.ConfigNamespaceName,
				network.LayeredID(network.ConfigOperator, network.LinkID(constants.KubeSpanLinkName)),
			),
			func(r resource.Resource) error {
				spec := r.(*network.LinkSpec).TypedSpec()

				spec.ConfigLayer = network.ConfigOperator
				spec.Name = constants.KubeSpanLinkName
				spec.Type = nethelpers.LinkNone
				spec.Kind = "wireguard"
				spec.Up = true
				spec.Logical = true

				spec.Wireguard = network.WireguardSpec{
					PrivateKey:   localSpec.PrivateKey,
					ListenPort:   constants.KubeSpanDefaultPort,
					FirewallMark: constants.KubeSpanDefaultFirewallMark,
					Peers:        wgPeers,
				}
				spec.Wireguard.Sort()

				return nil
			},
		); err != nil {
			return fmt.Errorf("error modifying link spec: %w", err)
		}

		if err = nfTablesMgr.Update(allowedIPsSet); err != nil {
			return fmt.Errorf("failed updating nftables: %w", err)
		}
	}
}

func (ctrl *ManagerController) cleanup(ctx context.Context, r controller.Runtime) error {
	for _, item := range []struct {
		namespace resource.Namespace
		typ       resource.Type
	}{
		{
			namespace: network.ConfigNamespaceName,
			typ:       network.LinkSpecType,
		},
		{
			namespace: network.ConfigNamespaceName,
			typ:       network.AddressSpecType,
		},
		{
			namespace: network.ConfigNamespaceName,
			typ:       network.RouteSpecType,
		},
		{
			namespace: kubespan.NamespaceName,
			typ:       kubespan.PeerStatusType,
		},
	} {
		// list keys for cleanup
		list, err := r.List(ctx, resource.NewMetadata(item.namespace, item.typ, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range list.Items {
			if res.Metadata().Owner() != ctrl.Name() {
				continue
			}

			if err = r.Destroy(ctx, res.Metadata()); err != nil {
				return fmt.Errorf("error cleaning up resource %s: %w", res, err)
			}
		}
	}

	return nil
}
