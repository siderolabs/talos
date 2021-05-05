// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package wglan

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/CyCoreSystems/netdiscover/discover"
	"github.com/hashicorp/go-multierror"
	"github.com/talos-systems/wglan-manager/types"
	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

const lastHandshakeTimeout = 5 * time.Minute

const peerCheckInterval = 10 * time.Second

const endpointRotationInterval = 5 * time.Second

const minimumReconcileInterval = 5 * time.Second

const maximumReconcileInterval = 5 * time.Minute

const reconciliationTimeout = 30 * time.Second

// A Controller provides a control interface for the Wireguard LAN system.
type Controller interface {
	// Start activates the Wireguard LAN system
	Start(ctx context.Context, r runtime.Runtime, logger *log.Logger, eg *errgroup.Group) error

	// Reconcile forces the Controller to validate and connect each peer
	Reconcile(ctx context.Context) error
}

// Config describes the configuration of the Wireguard LAN system.
type Config struct {
	// IP is the IP address of the Wireguard interface itself.
	IP netaddr.IPPrefix

	// Subnet defines an explicit subnet to be used for Wireguard.
	Subnet netaddr.IPPrefix

	// EnablePodRouting indicates that routes for Pod networks should also be added.
	EnablePodRouting bool

	// ForceLocalRoutes require all inter-node traffic to be encrypted, even when direct routes exist.
	ForceLocalRoutes bool

	// ClusterID indicates the unique identifier for the cluster for the purposes of Wireguard LAN discovery.
	// If provided, this ID must be a globally-unique name, so using a UUID is not a bad idea.
	// If NOT provided, a cryptographic hash of the cluster token will be used instead.
	ClusterID string

	// DiscoveryURL is the URL at which node and peer coordination occurs and from which the set of public keys of the member peers may be collected.
	DiscoveryURL string
}

// PrePeer describes a potential Wireguard Peer.
// Unlike a Wireguard Peer, it does not have a well-defined Endpoint.
// Instead, it contains a list of _candidate_ Endpoint IPs, through which the Peer Controller will cycle, until it finds on on which it can connect.
type PrePeer struct {
	PublicKey wgtypes.Key

	IP netaddr.IP

	NodeIPSets *NodeIPSets

	currentEndpoint netaddr.IPPort

	endpointChanged time.Time

	peerUp bool
}

// NodeIPSets provides a container for various types of IP Sets related to a Node.
type NodeIPSets struct {
	// SelfIPs is the list of IP addresses of the Node itself, either directly or indirectly (through NAT).
	SelfIPs []netaddr.IP

	// AssignedPrefixes is the list of IP prefixes which have been assigned to this Node.
	AssignedPrefixes *netaddr.IPSet

	// KnownEndpoints is the set of known endpoints for the Node.
	// These are IP:Port combinations which have thus far been known to work for at least one other peer.
	KnownEndpoints []netaddr.IPPort
}

// Merge combines additional IP Sets.
func (s *NodeIPSets) Merge(other *NodeIPSets) (changed bool, err error) {
	var epChanged, selfChanged bool

	if s.SelfIPs, selfChanged = mergeIPSets(s.SelfIPs, other.SelfIPs); selfChanged {
		changed = true
	}

	if s.KnownEndpoints, epChanged = mergeIPPortSets(s.KnownEndpoints, other.KnownEndpoints); epChanged {
		changed = true
	}

	if s.AssignedPrefixes == nil && other.AssignedPrefixes == nil {
		return
	}

	assignedIPBuilder := new(netaddr.IPSetBuilder)

	if s.AssignedPrefixes != nil {
		assignedIPBuilder.AddSet(s.AssignedPrefixes)
	}

	if other.AssignedPrefixes != nil {
		assignedIPBuilder.AddSet(other.AssignedPrefixes)
	}

	assignedIPSet, err := assignedIPBuilder.IPSet()
	if err != nil {
		return changed, fmt.Errorf("failed to build assigned IP set: %w", err)
	}

	if s.AssignedPrefixes == nil || !assignedIPSet.Equal(s.AssignedPrefixes) {
		changed = true

		s.AssignedPrefixes = assignedIPSet
	}

	return changed, nil
}

type wgLanController struct {
	nodeName string
	key      wgtypes.Key
	iface    *net.Interface

	clusterHash string

	clientProvider cluster.ClientProvider

	db *peerDB

	discoverers []Discoverer
	netdiscover discover.Discoverer

	logger *log.Logger

	reconcileSignal chan struct{}

	routingTable int

	cfg *Config
	wg  *wgtypes.Config
}

// New creates a new Wireguard LAN controller.
func New(iface string, wgLanConfig *Config, wgConfig *wgtypes.Config) (Controller, error) {
	var err error

	netIf, err := net.InterfaceByName(iface)
	if err != nil || netIf == nil {
		return nil, fmt.Errorf("failed to find interface %s by name: %w", iface, err)
	}

	clusterHash := sha256.Sum256([]byte(wgLanConfig.ClusterID))

	return &wgLanController{
		key:         *wgConfig.PrivateKey,
		cfg:         wgLanConfig,
		wg:          wgConfig,
		clusterHash: fmt.Sprintf("%x", clusterHash[:]),
		iface:       netIf,
		db:          new(peerDB),
		discoverers: []Discoverer{
			&externalDiscoverer{
				urlRoot: wgLanConfig.DiscoveryURL,
			},
			&kubeDiscoverer{
				includePodSubnets: wgLanConfig.EnablePodRouting,
			},
		},
		routingTable: constants.WireguardDefaultRoutingTable,
		netdiscover:  discover.NewDiscoverer(),
	}, nil
}

// Start implements the Controller interface.
func (c *wgLanController) Start(ctx context.Context, r runtime.Runtime, logger *log.Logger, eg *errgroup.Group) error {
	rulesManager := new(RulesManager)

	if err := rulesManager.Run(ctx, c); err != nil {
		return fmt.Errorf("failed to run rules manager: %w", err)
	}

	eg.Go(func() error {
		// Determine our own Node name
		nodeName, err := r.NodeName()
		if err != nil {
			return fmt.Errorf("failed to determine our own node name: %w", err)
		}

		c.nodeName = nodeName

		c.logger = logger

		c.reconcileSignal = make(chan struct{}, 1)

		c.maintain(ctx, logger)

		return nil
	})

	return nil
}

// Stop implements the Controller interface.
func (c *wgLanController) Stop() {
	if c.clientProvider != nil {
		c.clientProvider.Close() //nolint:errcheck
	}
}

// Start implements the Controller interface.
func (c *wgLanController) Reconcile(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, reconciliationTimeout)
	defer cancel()

	var merr *multierror.Error

	if err := c.registerSelf(ctx); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to register self: %w", err))
	}

	// Refresh existing peers:
	//   - add any new PrePeers from discovery sources
	//   - select next endpoint for disconnected existing peers
	//   - update each peer from amalgamation
	if err := c.updatePrePeers(ctx); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to update prepeers: %w", err))
	}

	peerConfigs, err := c.generatePeerConfigs()
	if err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to generate new peer configs: %w", err))
	}

	// Generate new wgconfig with updated Peers
	wgcfg := wgtypes.Config{
		ReplacePeers: false, // append-only
		Peers:        peerConfigs,
	}

	// Apply new wgconfig
	wc, err := wgctrl.New()
	if err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to create Wireguard client: %w", err))

		return merr.ErrorOrNil()
	}
	defer wc.Close() //nolint: errcheck

	if err := wc.ConfigureDevice(c.iface.Name, wgcfg); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to load updated peers into wireguard interface %q: %w", c.iface.Name, err))

		return merr.ErrorOrNil()
	}

	return merr.ErrorOrNil()
}

func (c *wgLanController) allDestinations() (*netaddr.IPSet, error) {
	b := new(netaddr.IPSetBuilder)

	for _, pp := range c.db.List() {
		if pp == nil || !pp.peerUp {
			continue
		}

		if routeSet, err := pp.routes(); err != nil {
			b.AddSet(routeSet)
		}
	}

	return b.IPSet()
}

func (c *wgLanController) registerSelf(ctx context.Context) error {
	var ips []netaddr.IP

	if ip, err := c.netdiscover.PrivateIPv4(); err == nil {
		if netIP, ok := netaddr.FromStdIP(ip); ok {
			ips = append(ips, netIP)
		}
	}

	if ip, err := c.netdiscover.PublicIPv4(); err == nil {
		if netIP, ok := netaddr.FromStdIP(ip); ok {
			ips = append(ips, netIP)
		}
	}

	if ip, err := c.netdiscover.PublicIPv6(); err == nil {
		if netIP, ok := netaddr.FromStdIP(ip); ok {
			ips = append(ips, netIP)
		}
	}

	n := &types.Node{
		ID:      c.key.PublicKey(),
		IP:      c.cfg.IP.IP(),
		Name:    c.nodeName,
		SelfIPs: ips,
	}

	var merr *multierror.Error

	for _, d := range c.discoverers {
		if err := d.Add(ctx, c.clusterHash, n); err != nil {
			merr = multierror.Append(merr, err)
		}
	}

	return merr.ErrorOrNil()
}

func (c *wgLanController) updatePrePeers(ctx context.Context) error {
	var merr *multierror.Error

	for i, d := range c.discoverers {
		ppList, err := d.List(ctx, c.clusterHash)
		if err != nil {
			merr = multierror.Append(merr, err)

			continue
		}

		for _, p := range ppList {
			if p.PublicKey == zeroKey {
				merr = multierror.Append(merr, fmt.Errorf("received empty prepeer from discoverer %d", i))

				continue
			}

			if p.PublicKey == c.key.PublicKey() {
				continue
			}

			if err := c.db.Merge(p); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to merge peer %q: %w", p.PublicKey.String(), err))
			}
		}
	}

	// Merge any existing wireguard peers into our PrePeer database
	peerList, err := c.getPeers()
	if err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to get current wireguard peer list: %w", err))
	} else if err := mergeExistingPeers(c.db, peerList); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to merge existing peers into PrePeer database: %w", err))
	}

	return merr.ErrorOrNil()
}

func (c *wgLanController) monitorPeers(ctx context.Context, logger *log.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(peerCheckInterval):
		}

		peerList, err := c.getPeers()
		if err != nil {
			logger.Println("failed to get Wireguard peers for state checking:", err)
		}

		for _, p := range peerList {
			if !peerIsUp(p) {
				pp := c.db.Get(p.PublicKey)
				if pp != nil {
					// NB: we can only do something about peers we know about
					logger.Println("at least one peer is down; signaling reconciliation run")

					select {
					case c.reconcileSignal <- struct{}{}:
					default:
					}
				}
			}
		}
	}
}

func (c *wgLanController) maintain(ctx context.Context, logger *log.Logger) {
	var err error

	cycleInterval := minimumReconcileInterval

	// TODO: hook up COSI resource monitoring for immediate reactions
	go c.monitorPeers(ctx, logger)

	// TODO: add a monitor for k8s Node adds/removes

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.reconcileSignal:
		case <-time.After(cycleInterval):
		}

		cycleInterval = maximumReconcileInterval

		if err = c.Reconcile(ctx); err != nil {
			cycleInterval = minimumReconcileInterval

			logger.Printf("wglan: failed to reconcile peer configuration for wglan %q: %s", c.iface.Name, err.Error())
		}
	}
}

func (c *wgLanController) getPeers() ([]wgtypes.Peer, error) {
	wc, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create Wireguard client: %w", err)
	}
	defer wc.Close() //nolint: errcheck

	d, err := wc.Device(c.iface.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to load wireguard device status: %w", err)
	}

	return d.Peers, nil
}

func (c *wgLanController) generatePeerConfigs() (out []wgtypes.PeerConfig, err error) {
	// Assemble PeerConfigs from PrePeers
	for _, pp := range c.db.List() {
		pc, err := pp.PeerConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to construct peer config for peer %q: %w", pp.PublicKey.String(), err)
		}

		if pp.PublicKey != c.key.PublicKey() {
			out = append(out, pc)
		}
	}

	return out, nil
}

func (p *PrePeer) routes() (*netaddr.IPSet, error) {
	if p == nil || p.NodeIPSets == nil {
		return nil, nil
	}

	b := netaddr.IPSetBuilder{}

	if p.NodeIPSets.AssignedPrefixes != nil {
		b.AddSet(p.NodeIPSets.AssignedPrefixes)
	}

	for _, ip := range p.NodeIPSets.SelfIPs {
		if !ip.IsZero() {
			b.Add(ip)
		}
	}

	return b.IPSet()
}

func (p *PrePeer) allowedIPs() (allowedIPs []netaddr.IPPrefix, err error) {
	if p == nil || p.NodeIPSets == nil {
		return nil, nil
	}

	b := netaddr.IPSetBuilder{}

	if !p.IP.IsZero() {
		b.Add(p.IP)
	}

	for _, ip := range p.NodeIPSets.SelfIPs {
		if !ip.IsZero() {
			b.Add(ip)
		}
	}

	if p.NodeIPSets.AssignedPrefixes != nil {
		b.AddSet(p.NodeIPSets.AssignedPrefixes)
	}

	set, err := b.IPSet()
	if err != nil {
		return nil, fmt.Errorf("failed to assemble allowed IP set: %w", err)
	}

	return set.Prefixes(), nil
}

// AllowedIPs returns the set of AllowedIPs for the PrePeer.
func (p *PrePeer) AllowedIPs() (allowedIPs []net.IPNet, err error) {
	allowed, err := p.allowedIPs()
	if err != nil {
		return nil, err
	}

	for _, ip := range allowed {
		allowedIPs = append(allowedIPs, *ip.IPNet())
	}

	return allowedIPs, nil
}

// SelectEndpoint retrieves the appropriate endpoint candidate based on the _current_ candidate and the amount of time that has elapsed since the candidate was last selected.
func (p *PrePeer) SelectEndpoint() {
	if p == nil ||
		p.NodeIPSets == nil {
		return
	}

	list := p.endpointCandidates(p.currentEndpoint)

	if p.endpointChanged.IsZero() || time.Since(p.endpointChanged) > endpointRotationInterval {
		p.endpointChanged = time.Now()

		p.currentEndpoint = p.nextEndpoint(list)
	}
}

func (p *PrePeer) endpointCandidates(currentEndpoint netaddr.IPPort) (out []netaddr.IPPort) {
	list := p.NodeIPSets.KnownEndpoints

	knownIPs := make([]netaddr.IPPort, 0, len(p.NodeIPSets.SelfIPs))

	for _, ip := range p.NodeIPSets.SelfIPs {
		knownIPs = append(knownIPs, netaddr.IPPortFrom(ip, uint16(constants.WireguardDefaultPort)))
	}

	list, _ = mergeIPPortSets(list, knownIPs)

	var index int

	for i, ip := range list {
		if ip == currentEndpoint {
			index = i
		}
	}

	return append(list[index:], list[:index]...)
}

func (p *PrePeer) nextEndpoint(list []netaddr.IPPort) (ep netaddr.IPPort) {
	if len(list) < 1 {
		return ep
	}

	if len(list) < 2 {
		return list[0]
	}

	for _, ip := range list[1:] {
		// Endpoint must not be inside the Wireguard subnet
		if p.IP == ip.IP() {
			continue
		}

		ep = ip
	}

	return ep
}

// PeerConfig returns the Wireguard Peer Config for the PrePeer's current state.
func (p *PrePeer) PeerConfig() (pc wgtypes.PeerConfig, err error) {
	keepAlive := constants.WireguardDefaultPeerKeepalive

	allowed, err := p.AllowedIPs()
	if err != nil {
		return pc, err
	}

	pc = wgtypes.PeerConfig{
		PublicKey:                   p.PublicKey,
		ReplaceAllowedIPs:           true,
		AllowedIPs:                  allowed,
		PersistentKeepaliveInterval: &keepAlive,
	}

	p.SelectEndpoint()

	if !p.currentEndpoint.IsZero() {
		pc.Endpoint = p.currentEndpoint.UDPAddr()
	}

	return pc, nil
}

// Merge adds data from a new PrePeer to an existing PrePeer.
func (p *PrePeer) Merge(other *PrePeer) (changed bool, err error) {
	return p.NodeIPSets.Merge(other.NodeIPSets)
}

func (p *PrePeer) endpointFromPeer(wgPeer wgtypes.Peer) (ep netaddr.IPPort, err error) {
	if wgPeer.Endpoint != nil {
		var ok bool

		ep, ok = netaddr.FromStdAddr(wgPeer.Endpoint.IP, wgPeer.Endpoint.Port, wgPeer.Endpoint.Zone)
		if !ok {
			return ep, fmt.Errorf("failed to parse wireguard endpoint %q", wgPeer.Endpoint.String())
		}
	}

	return ep, err
}

// MergeUpEndpoint updates a PrePeer with the given (expected to be
// up-and-working) Endpoint, adding it, if necessary, and modifying the port,
// if needed.
func (p *PrePeer) MergeUpEndpoint(upAddr *net.UDPAddr) error {
	if upAddr == nil {
		return nil
	}

	upEndpoint, ok := netaddr.FromStdAddr(upAddr.IP, upAddr.Port, upAddr.Zone)
	if !ok {
		return fmt.Errorf("failed to parse up endpoint address %q", upAddr.String())
	}

	p.endpointChanged = time.Now()

	p.currentEndpoint = upEndpoint

	mergeIPPortSets(p.NodeIPSets.KnownEndpoints, []netaddr.IPPort{upEndpoint})

	return nil
}

func peerIsUp(p wgtypes.Peer) bool {
	if p.LastHandshakeTime.IsZero() {
		return false
	}

	if time.Since(p.LastHandshakeTime) > lastHandshakeTimeout {
		return false
	}

	return true
}

// mergeExistingPeers merges a set of existing Peers from Wireguard into the set of PrePeers
// if and only if the PrePeer of the existing Peer already exists in the database.
func mergeExistingPeers(db *peerDB, peers []wgtypes.Peer) error {
	var err error

	var merr *multierror.Error

	for _, p := range peers {
		if pp := db.Get(p.PublicKey); pp != nil {
			pp.currentEndpoint, err = pp.endpointFromPeer(p)
			if err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to convert endpoint from existing wireguard peer %q: %w", p.PublicKey.String(), err))
			}

			pp.peerUp = peerIsUp(p)

			if !pp.peerUp {
				continue
			}

			// if the peer is up, make sure our PrePeer has its endpoint recorded
			if err := pp.MergeUpEndpoint(p.Endpoint); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to merge up endpoint for peer %q into PrePeer list: %w", p.PublicKey.String(), err))

				continue
			}
		}
	}

	return merr.ErrorOrNil()
}

// SubnetBitsMatch returns the subnet prefix bitlength of the first matching prefix if one exists.
// Otherwise, the address length is returned, indicating a subnet of size 1.
func SubnetBitsMatch(ip netaddr.IP, existing []netaddr.IPPrefix) uint8 {
	for _, existingIP := range existing {
		if existingIP.IP() == ip {
			return existingIP.Bits()
		}

		if existingIP.Contains(ip) {
			return existingIP.Bits()
		}
	}

	// We have no existing match, so just return the single-IP subnet mask size
	return ip.BitLen()
}
