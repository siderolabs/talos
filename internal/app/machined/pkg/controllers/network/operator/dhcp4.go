// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"go4.org/netipx"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// DHCP4 implements the DHCPv4 network operator.
type DHCP4 struct {
	logger *zap.Logger
	state  state.State

	linkName            string
	routeMetric         uint32
	skipHostnameRequest bool
	requestMTU          bool

	lease *nclient4.Lease

	mu          sync.Mutex
	addresses   []network.AddressSpecSpec
	links       []network.LinkSpecSpec
	routes      []network.RouteSpecSpec
	hostname    []network.HostnameSpecSpec
	resolvers   []network.ResolverSpecSpec
	timeservers []network.TimeServerSpecSpec
}

// NewDHCP4 creates DHCPv4 operator.
func NewDHCP4(logger *zap.Logger, linkName string, config network.DHCP4OperatorSpec, platform runtime.Platform, state state.State) *DHCP4 {
	return &DHCP4{
		logger:              logger,
		state:               state,
		linkName:            linkName,
		routeMetric:         config.RouteMetric,
		skipHostnameRequest: config.SkipHostnameRequest,
		// <3 azure
		// When including dhcp.OptionInterfaceMTU we don't get a dhcp offer back on azure.
		// So we'll need to explicitly exclude adding this option for azure.
		requestMTU: platform.Name() != "azure",
	}
}

// Prefix returns unique operator prefix which gets prepended to each spec.
func (d *DHCP4) Prefix() string {
	return fmt.Sprintf("dhcp4/%s", d.linkName)
}

// extractHostname extracts a hostname from the given resource if it is a valid network.HostnameStatus.
func extractHostname(res resource.Resource) network.HostnameStatusSpec {
	if res, ok := res.(*network.HostnameStatus); ok {
		return *res.TypedSpec()
	}

	return network.HostnameStatusSpec{}
}

// setupHostnameWatch returns the initial hostname and a channel that outputs all events related to hostname changes.
func (d *DHCP4) setupHostnameWatch(ctx context.Context) (<-chan state.Event, error) {
	hostnameWatchCh := make(chan state.Event)
	if err := d.state.Watch(ctx, resource.NewMetadata(
		network.NamespaceName,
		network.HostnameStatusType,
		network.HostnameID,
		resource.VersionUndefined,
	), hostnameWatchCh); err != nil {
		return nil, err
	}

	return hostnameWatchCh, nil
}

// knownHostname checks if the given hostname has been defined by this operator.
func (d *DHCP4) knownHostname(hostname network.HostnameStatusSpec) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i := range d.hostname {
		if d.hostname[i].FQDN() == hostname.FQDN() {
			return true
		}
	}

	return false
}

// waitForNetworkReady waits for the network to be ready and the leased address to
// be assigned to the associated so that unicast operations can bind successfully.
func (d *DHCP4) waitForNetworkReady(ctx context.Context) error {
	// If an IP address has been registered, wait for the address association to be ready
	if len(d.addresses) > 0 {
		_, err := d.state.WatchFor(ctx,
			resource.NewMetadata(
				network.NamespaceName,
				network.AddressStatusType,
				network.AddressID(d.linkName, d.addresses[0].Address),
				resource.VersionUndefined,
			),
			state.WithPhases(resource.PhaseRunning),
		)
		if err != nil {
			return fmt.Errorf("failed to wait for the address association to be ready: %w", err)
		}
	}

	// Wait for the network (address and connectivity) to be ready
	if err := network.NewReadyCondition(d.state, network.AddressReady, network.ConnectivityReady).Wait(ctx); err != nil {
		return fmt.Errorf("failed to wait for the network address and connectivity to be ready: %w", err)
	}

	return nil
}

// Run the operator loop.
//
//nolint:gocyclo,cyclop
func (d *DHCP4) Run(ctx context.Context, notifyCh chan<- struct{}) {
	const minRenewDuration = 5 * time.Second // Protect from renewing too often

	renewInterval := minRenewDuration

	// Never send the hostname on the first iteration, to have a chance to query the hostname from the DHCP server.
	// If the DHCP server doesn't provide a hostname, or if the hostname is overridden e.g. via machine config.
	// we'll restart the sequence and send the hostname.
	var hostname network.HostnameStatusSpec

	hostnameWatchCh, err := d.setupHostnameWatch(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		d.logger.Warn("failed to watch for hostname changes", zap.Error(err))
	}

	for {
		// Track if we need to acquire a new lease
		newLease := d.lease == nil

		// Perform a lease request or renewal
		leaseTime, err := d.requestRenew(ctx, hostname)
		if err != nil && !errors.Is(err, context.Canceled) {
			d.logger.Warn("request/renew failed", zap.Error(err), zap.String("link", d.linkName))
		}

		if err == nil {
			// Notify the underlying controller about the new lease
			if !channel.SendWithContext(ctx, notifyCh, struct{}{}) {
				return
			}

			if newLease {
				// Wait for networking to be established before transitioning to unicast operations
				if err = d.waitForNetworkReady(ctx); err != nil && !errors.Is(err, context.Canceled) {
					d.logger.Warn("failed to wait for networking to become ready", zap.Error(err))
				}
			}
		}

		if leaseTime > 0 {
			renewInterval = leaseTime / 2
		} else {
			renewInterval /= 2
		}

		renewInterval = max(renewInterval, minRenewDuration)

		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(renewInterval):
			case event := <-hostnameWatchCh:
				// Attempt to drain the hostname watch channel coalescing multiple events into a single
				// change to the DHCP.
			drainLoop:
				for {
					select {
					case event = <-hostnameWatchCh:
					case <-ctx.Done():
						return
					case <-time.After(time.Second):
						break drainLoop
					}
				}

				// If the hostname resource was deleted entirely, we must still inform the DHCP
				// server that the node has no hostname anymore. `extractHostname` will return a
				// blank hostname for a Tombstone resource generated by a deletion event.
				oldHostname := hostname
				hostname = extractHostname(event.Resource)

				d.logger.Debug("detected hostname change",
					zap.String("old", oldHostname.FQDN()),
					zap.String("new", hostname.FQDN()),
				)

				// If, on first invocation, the DHCP server has given a new hostname for the node,
				// and the `network.HostnameSpecController` decides to apply it as a preferred
				// hostname, this operator would unnecessarily drop the lease and restart DHCP
				// discovery. Thus, if the selected hostname has been sourced from this operator,
				// we don't need to do anything.
				if (oldHostname == network.HostnameStatusSpec{} && d.knownHostname(hostname)) || oldHostname == hostname {
					continue
				}

				// While updating the hostname together with a RENEW request works with dnsmasq, it
				// doesn't work with the Windows Server DHCP + DNS. A hostname update via an
				// INIT-REBOOT request also gets ignored. Thus, the only reliable way to update the
				// hostname seems to be to forget the old release and initiate a new DISCOVER flow
				// with the new hostname. RFC 2131 doesn't define any better way to do this, and,
				// as a DISCOVER request cannot be targeted at the previous lessor according to the
				// spec, the node may switch DHCP servers on hostname change. However, this is not
				// a major concern, since a single network should not host multiple competing DHCP
				// servers in the first place.
				d.lease = nil

				d.logger.Debug("restarting DHCP sequence due to hostname change",
					zap.Strings("dhcp_hostname", xslices.Map(d.hostname, func(spec network.HostnameSpecSpec) string {
						return spec.Hostname
					})),
				)
			}

			break
		}
	}
}

// AddressSpecs implements Operator interface.
func (d *DHCP4) AddressSpecs() []network.AddressSpecSpec {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.addresses
}

// LinkSpecs implements Operator interface.
func (d *DHCP4) LinkSpecs() []network.LinkSpecSpec {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.links
}

// RouteSpecs implements Operator interface.
func (d *DHCP4) RouteSpecs() []network.RouteSpecSpec {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.routes
}

// HostnameSpecs implements Operator interface.
func (d *DHCP4) HostnameSpecs() []network.HostnameSpecSpec {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.hostname
}

// ResolverSpecs implements Operator interface.
func (d *DHCP4) ResolverSpecs() []network.ResolverSpecSpec {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.resolvers
}

// TimeServerSpecs implements Operator interface.
func (d *DHCP4) TimeServerSpecs() []network.TimeServerSpecSpec {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.timeservers
}

//nolint:gocyclo
func (d *DHCP4) parseNetworkConfigFromAck(ack *dhcpv4.DHCPv4, useHostname bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	addr, _ := netipx.FromStdIPNet(&net.IPNet{
		IP:   ack.YourIPAddr,
		Mask: ack.SubnetMask(),
	})

	d.addresses = []network.AddressSpecSpec{
		{
			Address:     addr,
			LinkName:    d.linkName,
			Family:      nethelpers.FamilyInet4,
			Scope:       nethelpers.ScopeGlobal,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			ConfigLayer: network.ConfigOperator,
		},
	}

	mtu, err := dhcpv4.GetUint16(dhcpv4.OptionInterfaceMTU, ack.Options)
	if err == nil {
		d.links = []network.LinkSpecSpec{
			{
				Name: d.linkName,
				MTU:  uint32(mtu),
				Up:   true,
			},
		}
	} else {
		d.links = nil
	}

	// rfc3442:
	//   If the DHCP server returns both a Classless Static Routes option and
	//   a Router option, the DHCP client MUST ignore the Router option.
	d.routes = nil

	if len(ack.ClasslessStaticRoute()) > 0 {
		for _, route := range ack.ClasslessStaticRoute() {
			gw, _ := netipx.FromStdIP(route.Router)
			dst, _ := netipx.FromStdIPNet(route.Dest)

			d.routes = append(d.routes, network.RouteSpecSpec{
				Family:      nethelpers.FamilyInet4,
				Destination: dst,
				Source:      addr.Addr(),
				Gateway:     gw,
				OutLinkName: d.linkName,
				Table:       nethelpers.TableMain,
				Priority:    d.routeMetric,
				Scope:       nethelpers.ScopeGlobal,
				Type:        nethelpers.TypeUnicast,
				Protocol:    nethelpers.ProtocolBoot,
				ConfigLayer: network.ConfigOperator,
			})
		}
	} else {
		for _, router := range ack.Router() {
			gw, _ := netipx.FromStdIP(router)

			d.routes = append(d.routes, network.RouteSpecSpec{
				Family:      nethelpers.FamilyInet4,
				Gateway:     gw,
				Source:      addr.Addr(),
				OutLinkName: d.linkName,
				Table:       nethelpers.TableMain,
				Priority:    d.routeMetric,
				Scope:       nethelpers.ScopeGlobal,
				Type:        nethelpers.TypeUnicast,
				Protocol:    nethelpers.ProtocolBoot,
				ConfigLayer: network.ConfigOperator,
			})

			if !addr.Contains(gw) {
				// Add an interface route for the gateway if it's not in the same network
				d.routes = append(d.routes, network.RouteSpecSpec{
					Family:      nethelpers.FamilyInet4,
					Destination: netip.PrefixFrom(gw, gw.BitLen()),
					Source:      addr.Addr(),
					OutLinkName: d.linkName,
					Table:       nethelpers.TableMain,
					Priority:    d.routeMetric,
					Scope:       nethelpers.ScopeLink,
					Type:        nethelpers.TypeUnicast,
					Protocol:    nethelpers.ProtocolBoot,
					ConfigLayer: network.ConfigOperator,
				})
			}
		}
	}

	for i := range d.routes {
		d.routes[i].Normalize()
	}

	if useHostname {
		d.hostname = nil

		if ack.HostName() != "" {
			spec := network.HostnameSpecSpec{
				ConfigLayer: network.ConfigOperator,
			}

			if err := spec.ParseFQDN(ack.HostName()); err == nil {
				if ack.DomainName() != "" {
					spec.Domainname = ack.DomainName()
				}

				d.hostname = []network.HostnameSpecSpec{
					spec,
				}
			}
		}
	}

	if len(ack.DNS()) > 0 {
		convertIP := func(ip net.IP) netip.Addr {
			result, _ := netipx.FromStdIP(ip)

			return result
		}

		d.resolvers = []network.ResolverSpecSpec{
			{
				DNSServers:  xslices.Map(ack.DNS(), convertIP),
				ConfigLayer: network.ConfigOperator,
			},
		}
	} else {
		d.resolvers = nil
	}

	if len(ack.NTPServers()) > 0 {
		convertIP := func(ip net.IP) string {
			result, _ := netipx.FromStdIP(ip)

			return result.String()
		}

		d.timeservers = []network.TimeServerSpecSpec{
			{
				NTPServers:  xslices.Map(ack.NTPServers(), convertIP),
				ConfigLayer: network.ConfigOperator,
			},
		}
	} else {
		d.timeservers = nil
	}
}

func (d *DHCP4) newClient() (*nclient4.Client, error) {
	var clientOpts []nclient4.ClientOpt

	// We have an existing lease, target the server with unicast
	if d.lease != nil && !d.lease.ACK.ServerIPAddr.IsUnspecified() {
		// RFC 2131, section 4.3.2:
		//     DHCPREQUEST generated during RENEWING state:
		//     ... This message will be unicast, so no relay
		//     agents will be involved in its transmission.
		clientOpts = append(clientOpts,
			nclient4.WithServerAddr(&net.UDPAddr{
				IP:   d.lease.ACK.ServerIPAddr,
				Port: nclient4.ServerPort,
			}),
			// WithUnicast must be specified manually, WithServerAddr is not enough
			nclient4.WithUnicast(&net.UDPAddr{
				IP:   d.lease.ACK.YourIPAddr,
				Port: nclient4.ClientPort,
			}),
		)
	}

	// Create a new client, the caller is responsible for closing it
	return nclient4.New(d.linkName, clientOpts...)
}

//nolint:gocyclo
func (d *DHCP4) requestRenew(ctx context.Context, hostname network.HostnameStatusSpec) (time.Duration, error) {
	opts := []dhcpv4.OptionCode{
		dhcpv4.OptionClasslessStaticRoute,
		dhcpv4.OptionDomainNameServer,
		// TODO(twelho): This is unused until network.ResolverSpec supports search domains
		dhcpv4.OptionDNSDomainSearchList,
		dhcpv4.OptionNTPServers,
	}

	if d.requestMTU {
		opts = append(opts, dhcpv4.OptionInterfaceMTU)
	}

	sendHostnameRequest := !d.skipHostnameRequest
	if hostname.Hostname != "" && !d.knownHostname(hostname) {
		// If we are supposed to publish a hostname, don't request one from the DHCP server.
		//
		// DHCP hostname parroting protection: if, e.g., `dnsmasq` receives a request that both
		// sends a hostname and requests one, it will "parrot" the sent hostname back if no other
		// name has been defined for the requesting host. This causes update anomalies, since
		// removing a hostname defined previously by, e.g., the configuration layer, causes a copy
		// of that hostname to live on in a spec defined by this operator, even though it isn't
		// sourced from DHCP.
		//
		// To avoid this issue, never send and request a hostname in the same operation. When
		// negotiating a new lease, first send the current hostname when acquiring the lease, and
		// then follow up with a dedicated INFORM request asking the server for a DHCP-defined
		// hostname. When renewing a lease, we're free to always request a hostname with an INFORM
		// (to detect server-side changes), since any changes to the node hostname will cause a
		// lease invalidation and re-start the negotiation process. More details below.
		sendHostnameRequest = false
	}

	if sendHostnameRequest {
		opts = append(opts, dhcpv4.OptionHostName, dhcpv4.OptionDomainName)
	}

	mods := []dhcpv4.Modifier{dhcpv4.WithRequestedOptions(opts...)}

	if !sendHostnameRequest {
		// If the node has a hostname, always send it to the DHCP
		// server with option 12 during lease acquisition and renewal
		if len(hostname.Hostname) > 0 {
			mods = append(mods, dhcpv4.WithOption(dhcpv4.OptHostName(hostname.Hostname)))
		}

		if len(hostname.Domainname) > 0 {
			mods = append(mods, dhcpv4.WithOption(dhcpv4.OptDomainName(hostname.Domainname)))
		}
	}

	client, err := d.newClient()
	if err != nil {
		return 0, err
	}

	//nolint:errcheck
	defer client.Close()

	switch {
	case d.lease != nil && !d.lease.ACK.ServerIPAddr.IsUnspecified():
		d.logger.Debug("DHCP RENEW", zap.String("link", d.linkName))
		d.lease, err = client.Renew(ctx, d.lease, mods...)
	case d.lease != nil && d.lease.Offer != nil:
		d.logger.Debug("DHCP REQUEST FROM OFFER", zap.String("link", d.linkName))
		d.lease, err = client.RequestFromOffer(ctx, d.lease.Offer, mods...)
	default:
		d.logger.Debug("DHCP REQUEST", zap.String("link", d.linkName))
		d.lease, err = client.Request(ctx, mods...)
	}

	if err != nil {
		// explicitly clear the lease on failure to start with the discovery sequence next time
		d.lease = nil

		return 0, err
	}

	d.logger.Debug("DHCP ACK", zap.String("link", d.linkName), zap.String("dhcp", collapseSummary(d.lease.ACK.Summary())))

	d.parseNetworkConfigFromAck(d.lease.ACK, sendHostnameRequest)

	return d.lease.ACK.IPAddressLeaseTime(time.Minute * 30), nil
}

func collapseSummary(summary string) string {
	lines := strings.Split(summary, "\n")[1:]

	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}

	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return strings.Join(lines, ", ")
}
