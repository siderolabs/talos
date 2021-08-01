// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package wglan

import (
	"context"
	"fmt"
	"time"

	"github.com/CyCoreSystems/netdiscover/discover"
	"github.com/hashicorp/go-multierror"
	"github.com/talos-systems/wglan-manager/types"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/network"
)

// WG whitepaper defines a downed peer as being:
// Keepalive Timeout (25s) + Rekey Timeout (5s) + Rekey Attempt Timeout (90s)
//nolint: durationcheck
const peerDownTimeout = (25 + 5 + 90) * time.Second

const endpointRotationInterval = 5 * time.Second

// MinimumReconcileInterval is the minimum cycle time for WgLAN reconciliation.
const MinimumReconcileInterval = 5 * time.Second

// MaximumReconcileInterval is the maximum cycle time for WgLAN reconciliation.
const MaximumReconcileInterval = 5 * time.Minute

const reconciliationTimeout = 30 * time.Second

// NodenameFunc is a function which should return the Kubernetes Nodename, when it is available.
type NodenameFunc func() string

// PeerManager maintains the database of WgLAN Peers.
type PeerManager struct {
	Config *Config

	// clientProvider cluster.ClientProvider

	db *PeerDB

	registries  []Registry
	netdiscover discover.Discoverer

	logger *zap.Logger
}

// Config describes the configuration of the Wireguard LAN system.
type Config struct {
	// ClusterID indicates the unique identifier for the cluster for the purposes of Wireguard LAN discovery.
	// If provided, this ID must be a globally-unique name, so using a UUID is not a bad idea.
	ClusterID string

	// DiscoveryURL is the URL at which node and peer coordination occurs and from which the set of public keys of the member peers may be collected.
	DiscoveryURL string

	// EnablePodRouting indicates that routes for Pod networks should also be added.
	EnablePodRouting bool

	// ForceLocalRoutes require all inter-node traffic to be encrypted, even when direct routes exist.
	ForceLocalRoutes bool

	// IP is the IP address of the Wireguard interface itself.
	IP netaddr.IPPrefix

	// LinkName is the name of the Wireguard interface.
	LinkName string

	// Nodename is a function which should returnt he Kubernetes Nodename, when it is available.
	Nodename NodenameFunc

	// PublicKey is the public key of the Wireguard interface.
	PublicKey string

	// RoutingTable is the table to be used for routing destinations to the Wiregurd interface.
	RoutingTable int

	// Subnet defines an explicit subnet to be used for Wireguard.
	Subnet netaddr.IPPrefix
}

// Peer describes a potential Wireguard Peer.
// Unlike a Wireguard Peer, it does not have a well-defined Endpoint.
// Instead, it contains a list of _candidate_ Endpoint IPs, through which the Peer Controller will cycle, until it finds one to which it can connect.
type Peer struct {
	node *types.Node

	assignedPrefixes *netaddr.IPSet

	currentEndpoint netaddr.IPPort

	endpointChanged time.Time

	peerUp bool
}

// PublicKey returns the PublicKey of the Peer.
func (p *Peer) PublicKey() string {
	return p.node.ID
}

// PossibleEndpoints describes the set of potential endpoints for contacting the given node.
// If defaultPort == 0, the global system default will be used.
func (p *Peer) PossibleEndpoints(defaultPort uint16) (out []netaddr.IPPort) {
	for _, a := range p.node.Addresses {
		port := a.Port

		if a.Port == 0 {
			port = constants.WireguardDefaultPort
		}

		if !a.IP.IsZero() {
			out = append(out, netaddr.IPPortFrom(a.IP, port))

			continue
		}

		if a.Name == "" {
			continue
		}

		// TODO: wireguard defines its own DNS-based service discovery system using SRV and PTR records, which we should implement at some point.

		ips, err := resolveHostname(a.Name, port)
		if err != nil {
			// NB: it is expected that hostnames may often not be resolvable
			continue
		}

		out = append(out, ips...)
	}

	return out
}

// AllowedPrefixes describes the set of prefixes for addresses which should be allowed to be received from this Node.
func (p *Peer) AllowedPrefixes() (*netaddr.IPSet, error) {
	set := new(netaddr.IPSetBuilder)

	set.Add(p.node.IP)

	for _, a := range p.node.Addresses {
		if !a.IP.IsZero() {
			set.Add(a.IP)
		}
	}

	set.AddSet(p.assignedPrefixes)

	return set.IPSet()
}

// Merge a peer with another peer of the same ID.
func (p *Peer) Merge(other *Peer) (err error) {
	if p.PublicKey() != other.PublicKey() {
		return fmt.Errorf("peer IDs do not match (%q vs %q)", p.PublicKey(), other.PublicKey())
	}

	p.node.AddAddresses(other.node.Addresses...)

	prefixSet := new(netaddr.IPSetBuilder)

	if p.assignedPrefixes != nil {
		prefixSet.AddSet(p.assignedPrefixes)
	}

	if other.assignedPrefixes != nil {
		prefixSet.AddSet(other.assignedPrefixes)
	}

	p.assignedPrefixes, err = prefixSet.IPSet()
	if err != nil {
		return fmt.Errorf("failed to build assigned IP set: %w", err)
	}

	return nil
}

// SelectEndpoint retrieves the appropriate endpoint candidate based on the _current_ candidate and the amount of time that has elapsed since the candidate was last selected.
func (p *Peer) SelectEndpoint(defaultPort uint16) error {
	if p == nil {
		return fmt.Errorf("peer is nil")
	}

	if p.peerUp {
		return nil
	}

	if p.endpointChanged.IsZero() || time.Since(p.endpointChanged) > endpointRotationInterval {
		ep, err := p.nextEndpoint(defaultPort)
		if err != nil {
			return fmt.Errorf("failed to select endpoint: %w", err)
		}

		p.endpointChanged = time.Now()

		p.currentEndpoint = ep
	}

	return nil
}

func (p *Peer) nextEndpoint(defaultPort uint16) (ep netaddr.IPPort, err error) {
	list := p.PossibleEndpoints(defaultPort)

	if len(list) < 1 {
		return ep, fmt.Errorf("no endpoints available")
	}

	for i, ip := range list {
		if p.currentEndpoint.IP() == ip.IP() {
			if len(list) > i+2 {
				return list[i+1], nil
			}

			return list[0], nil
		}
	}

	return list[0], nil
}

// PeerConfig returns the Wireguard Peer Config for the Peer's current state.
func (p *Peer) PeerConfig(defaultPort uint16) (pc network.WireguardPeer, err error) {
	keepAlive := constants.WireguardDefaultPeerKeepalive

	allowed, err := p.AllowedPrefixes()
	if err != nil {
		return pc, err
	}

	pc = network.WireguardPeer{
		PublicKey:                   p.PublicKey(),
		AllowedIPs:                  allowed.Prefixes(),
		PersistentKeepaliveInterval: keepAlive,
	}

	if err = p.SelectEndpoint(defaultPort); err != nil {
		return pc, fmt.Errorf("failed to select endpoint for peer %q: %w", p.node.Name, err)
	}

	if !p.currentEndpoint.IsZero() {
		pc.Endpoint = p.currentEndpoint.String()
	}

	return pc, nil
}

// NewPeerManager returns a WgLAN Peer manager, which maintains the list of available Peers in the given database.
func NewPeerManager(cfg *Config, db *PeerDB) *PeerManager {
	return &PeerManager{
		Config: cfg,
		db:     db,
		registries: []Registry{
			&RegistryExternal{
				URLRoot: cfg.DiscoveryURL,
			},
			&RegistryKubernetes{
				IncludePodSubnets: cfg.EnablePodRouting,
			},
		},
		netdiscover: discover.NewDiscoverer(),
	}
}

// Reconcile interacts with each registry, registering and updating the Peers.
func (m *PeerManager) Reconcile(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, reconciliationTimeout)
	defer cancel()

	var merr *multierror.Error

	if err := m.registerSelf(ctx); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to register self: %w", err))
	}

	// Refresh existing peers:
	//   - add any new Peers from discovery sources
	//   - select next endpoint for disconnected existing peers
	//   - update each peer from amalgamation
	if err := m.updatePeers(ctx); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to update peers: %w", err))
	}

	return merr.ErrorOrNil()
}

//nolint: gocyclo
func (m *PeerManager) registerSelf(ctx context.Context) error {
	var addrs []*types.Address

	if ip, err := m.netdiscover.PrivateIPv4(); err == nil {
		if netIP, ok := netaddr.FromStdIP(ip); ok {
			addrs = append(addrs, &types.Address{
				IP:           netIP,
				LastReported: time.Now(),
			})
		}
	}

	if ip, err := m.netdiscover.PublicIPv4(); err == nil {
		if netIP, ok := netaddr.FromStdIP(ip); ok {
			addrs = append(addrs, &types.Address{
				IP:           netIP,
				LastReported: time.Now(),
			})
		}
	}

	if ip, err := m.netdiscover.PublicIPv6(); err == nil {
		if netIP, ok := netaddr.FromStdIP(ip); ok {
			addrs = append(addrs, &types.Address{
				IP:           netIP,
				LastReported: time.Now(),
			})
		}
	}

	if addr, err := m.netdiscover.Hostname(); err == nil {
		if addr != "" {
			addrs = append(addrs, &types.Address{
				Name:         addr,
				LastReported: time.Now(),
			})
		}
	}

	n := &types.Node{
		ID:        m.Config.PublicKey,
		IP:        m.Config.IP.IP(),
		Name:      m.Config.Nodename(),
		Addresses: addrs,
	}

	var merr *multierror.Error

	for _, r := range m.registries {
		if err := r.Add(ctx, m.Config.ClusterID, n); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("registration of node %q to registry %q failed: %w", n.Name, r.Name(), err))
		}
	}

	return merr.ErrorOrNil()
}

func (m *PeerManager) updatePeers(ctx context.Context) error {
	var merr *multierror.Error

	m.logger.Sugar().Debugf("updating peers for cluster %q", m.Config.ClusterID)

	// Merge Peers from Registries
	for _, r := range m.registries {
		ppList, err := r.List(ctx, m.Config.ClusterID)
		if err != nil {
			merr = multierror.Append(merr, err)

			continue
		}

		if len(ppList) == 0 {
			merr = multierror.Append(merr, fmt.Errorf("no peers found in registry %q", r.Name()))
		}

		m.logger.Sugar().Debugf("received %d peers from registry %q", len(ppList), r.Name())

		for _, p := range ppList {
			if p.PublicKey() == "" {
				merr = multierror.Append(merr, fmt.Errorf("received empty peer from registry %q", r.Name()))

				continue
			}

			if p.PublicKey() == m.Config.PublicKey {
				continue
			}

			if err := m.db.Merge(p); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to merge peer %q: %w", p.PublicKey(), err))
			}
		}
	}

	// Merge existing Peers from the Wireguard interface
	peerList, err := m.getWGPeers()
	if err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to get current wireguard peer list: %w", err))
	} else if err := mergeExistingPeers(m.db, peerList); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to merge existing peers into Peer database: %w", err))
	}

	return merr.ErrorOrNil()
}

// Run starts the PeerMananager, keeping the database of Peers up to date.
func (m *PeerManager) Run(ctx context.Context, logger *zap.Logger) error {
	var err error

	cycleInterval := MinimumReconcileInterval

	m.logger = logger

	// TODO: add a monitor for k8s Node adds/removes

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(cycleInterval):
		}

		cycleInterval = MaximumReconcileInterval

		if err = m.Reconcile(ctx); err != nil {
			cycleInterval = MinimumReconcileInterval

			logger.Sugar().Infof("failed to reconcile peer configuration for wglan %q: %s", m.Config.LinkName, err.Error())
		}
	}
}

func (m *PeerManager) getWGPeers() ([]wgtypes.Peer, error) {
	wc, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create Wireguard client: %w", err)
	}
	defer wc.Close() //nolint: errcheck

	d, err := wc.Device(m.Config.LinkName)
	if err != nil {
		return nil, fmt.Errorf("failed to load wireguard device (%q) status: %w", m.Config.LinkName, err)
	}

	return d.Peers, nil
}

func endpointFromWGPeer(wgPeer wgtypes.Peer) (ep netaddr.IPPort, err error) {
	if wgPeer.Endpoint != nil {
		var ok bool

		ep, ok = netaddr.FromStdAddr(wgPeer.Endpoint.IP, wgPeer.Endpoint.Port, wgPeer.Endpoint.Zone)
		if !ok {
			return ep, fmt.Errorf("failed to parse wireguard endpoint %q", wgPeer.Endpoint.String())
		}
	}

	return ep, err
}

func peerIsUp(p wgtypes.Peer) bool {
	if p.LastHandshakeTime.IsZero() {
		return false
	}

	if time.Since(p.LastHandshakeTime) > peerDownTimeout {
		return false
	}

	return true
}

// mergeExistingPeers merges a set of existing Peers from Wireguard into the set of Peers
// if and only if the Peer of the existing Peer already exists in the database.
func mergeExistingPeers(db *PeerDB, peers []wgtypes.Peer) error {
	var err error

	var merr *multierror.Error

	var peerCount, peerUpCount float64

	for _, p := range peers {
		if pp := db.Get(p.PublicKey); pp != nil {
			pp.currentEndpoint, err = endpointFromWGPeer(p)
			if err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to convert endpoint from existing wireguard peer %q: %w", p.PublicKey.String(), err))
			}

			peerCount++

			if pp.peerUp = peerIsUp(p); pp.peerUp {
				peerUpCount++
			}
		}
	}

	metricPeerCount.Set(peerCount)

	metricPeerUpCount.Set(peerUpCount)

	return merr.ErrorOrNil()
}
