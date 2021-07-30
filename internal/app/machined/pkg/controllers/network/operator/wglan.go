// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operator

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"time"

	"github.com/mdlayher/netx/eui64"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network/operator/wglan"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/resources/network"
)

// WgLAN implements a network operator for controlling the Wireguard LAN system.
type WgLAN struct {
	Config *wglan.Config

	privateKey string

	clusterHash string
	fwMark      int
	listenPort  uint16

	db *wglan.PeerDB

	logger *zap.Logger
}

// NewWgLAN creates a Wireguard LAN operator.
func NewWgLAN(logger *zap.Logger, hostnamer wglan.Hostnamer, linkName string, prefix netaddr.IPPrefix, clusterID string, privateKey string, discoveryURL string, podNetworking bool) *WgLAN {
	if discoveryURL == "" {
		discoveryURL = constants.WireguardDefaultNATDiscoveryService
	}

	privKey, err := wgtypes.ParseKey(privateKey)
	if err != nil {
		logger.Sugar().Fatalf("failed to parse Wireguard private key %q: %w", privateKey, err)
	}

	ip, err := wgEUI64(prefix)
	if err != nil {
		logger.Warn("failed to generate local IP address", zap.Error(err))

		return nil
	}

	pubKey := privKey.PublicKey().String()

	clusterHash := sha256.Sum256([]byte(clusterID))

	cfg := &wglan.Config{
		ClusterID:        clusterID,
		DiscoveryURL:     discoveryURL,
		EnablePodRouting: podNetworking,
		IP:               ip,
		LinkName:         linkName,
		Hostnamer:        hostnamer,
		PublicKey:        pubKey,
		RoutingTable:     constants.WireguardDefaultRoutingTable,
		Subnet:           prefix,
	}

	return &WgLAN{
		Config:      cfg,
		logger:      logger,
		clusterHash: fmt.Sprintf("%x", clusterHash[:]),
		privateKey:  privateKey,
		db:          new(wglan.PeerDB),
	}
}

// Prefix returns unique operator prefix which gets prepended to each spec.
func (o *WgLAN) Prefix() string {
	return fmt.Sprintf("wglan/%s", o.Config.LinkName)
}

// Run the operator loop.
func (o *WgLAN) Run(ctx context.Context, notifyCh chan<- struct{}) {
	var rulesManager *wglan.RulesManager

	var peerManager *wglan.PeerManager

	for ctx.Err() == nil {
		time.Sleep(time.Second)

		rulesManager = new(wglan.RulesManager)

		if err := rulesManager.Run(ctx, o.logger, o.db); err != nil {
			o.logger.Warn("failed to start rules manager", zap.Error(err))

			continue
		}

		peerManager = wglan.NewPeerManager(o.Config, o.db)

		if err := peerManager.Run(ctx, o.logger); err != nil {
			o.logger.Warn("failed to start peer manager", zap.Error(err))
		}
	}
}

// AddressSpecs implements Operator interface.
func (o *WgLAN) AddressSpecs() []network.AddressSpecSpec {
	return []network.AddressSpecSpec{
		{
			Address:         o.Config.IP,
			LinkName:        o.Config.LinkName,
			Family:          unix.AF_INET6,
			Scope:           unix.RT_SCOPE_UNIVERSE,
			Flags:           0,
			AnnounceWithARP: false,
			ConfigLayer:     network.ConfigOperator,
		},
	}
}

// LinkSpecs implements Operator interface.
func (o *WgLAN) LinkSpecs() []network.LinkSpecSpec {
	o.logger.Info("returning link specs...")

	return []network.LinkSpecSpec{
		{
			Name:       o.Config.LinkName,
			Logical:    true,
			Up:         true,
			Kind:       network.LinkKindWireguard,
			Type:       nethelpers.LinkNetrom,
			VLAN:       network.VLANSpec{},
			BondMaster: network.BondMasterSpec{},
			Wireguard: network.WireguardSpec{
				PrivateKey:   o.privateKey,
				ListenPort:   int(o.listenPort),
				FirewallMark: o.fwMark,
				Peers:        o.getPeers(o.listenPort),
			},
			ConfigLayer: network.ConfigOperator,
		},
	}
}

// RouteSpecs implements Operator interface.
func (o *WgLAN) RouteSpecs() []network.RouteSpecSpec {
	return []network.RouteSpecSpec{
		{
			Family:      unix.AF_INET,
			Destination: netaddr.MustParseIPPrefix("0.0.0.0/0"),
			Source:      netaddr.IPPrefix{},
			Gateway:     netaddr.IP{},
			OutLinkName: o.Config.LinkName,
			Table:       nethelpers.RoutingTable(o.Config.RoutingTable),
			Priority:    1,
			Scope:       unix.RT_SCOPE_UNIVERSE,
			Type:        unix.RTN_UNICAST,
			Flags:       0,
			Protocol:    unix.RTPROT_STATIC,
			ConfigLayer: network.ConfigOperator,
		},
		{
			Family:      unix.AF_INET6,
			Destination: netaddr.MustParseIPPrefix("::/0"),
			Source:      netaddr.IPPrefix{},
			Gateway:     netaddr.IP{},
			OutLinkName: o.Config.LinkName,
			Table:       nethelpers.RoutingTable(o.Config.RoutingTable),
			Priority:    1,
			Scope:       unix.RT_SCOPE_UNIVERSE,
			Type:        unix.RTN_UNICAST,
			Flags:       0,
			Protocol:    unix.RTPROT_STATIC,
			ConfigLayer: network.ConfigOperator,
		},
	}
}

// HostnameSpecs implements Operator interface.
func (o *WgLAN) HostnameSpecs() []network.HostnameSpecSpec {
	return nil
}

// ResolverSpecs implements Operator interface.
func (o *WgLAN) ResolverSpecs() []network.ResolverSpecSpec {
	return nil
}

// TimeServerSpecs implements Operator interface.
func (o *WgLAN) TimeServerSpecs() []network.TimeServerSpecSpec {
	return nil
}

func (o *WgLAN) getPeers(defaultPort uint16) (out []network.WireguardPeer) {
	for _, pp := range o.db.List() {
		pc, err := pp.PeerConfig(defaultPort)
		if err != nil {
			o.logger.Warn("failed to construct peer config",
				zap.String("peer", pp.PublicKey()),
				zap.Error(err),
			)
		}

		if pp.PublicKey() != o.Config.PublicKey {
			out = append(out, pc)
		}
	}

	if len(out) == 0 {
		o.logger.Info("no peers found in local WgLAN database")
	}

	return out
}

func wgEUI64(prefix netaddr.IPPrefix) (out netaddr.IPPrefix, err error) {
	mac, err := firstRealMAC()
	if err != nil {
		return out, fmt.Errorf("failed to find first MAC address: %w", err)
	}

	stdIP, err := eui64.ParseMAC(prefix.IPNet().IP, mac)
	if err != nil {
		return out, fmt.Errorf("failed to parse MAC into EUI-64 address: %w", err)
	}

	ip, ok := netaddr.FromStdIP(stdIP)
	if !ok {
		return out, fmt.Errorf("failed to parse intermediate standard IP %q: %w", stdIP.String(), err)
	}

	return netaddr.IPPrefixFrom(ip, prefix.Bits()), nil
}

func firstRealMAC() (net.HardwareAddr, error) {
	h, err := netlink.NewHandle(0)
	if err != nil {
		return nil, fmt.Errorf("failed to get netlink handle: %w", err)
	}

	list, err := h.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to get list of links: %w", err)
	}

	for _, l := range list {
		if l.Type() == "device" && l.Attrs().Flags&net.FlagLoopback != net.FlagLoopback {
			return l.Attrs().HardwareAddr, nil
		}
	}

	return nil, fmt.Errorf("no physical NICs found")
}
