// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"go.uber.org/zap"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// DHCP4 implements the DHCPv4 network operator.
type DHCP4 struct {
	logger *zap.Logger

	linkName    string
	routeMetric uint32
	requestMTU  bool

	offer *dhcpv4.DHCPv4

	mu          sync.Mutex
	addresses   []network.AddressSpecSpec
	links       []network.LinkSpecSpec
	routes      []network.RouteSpecSpec
	hostname    []network.HostnameSpecSpec
	resolvers   []network.ResolverSpecSpec
	timeservers []network.TimeServerSpecSpec
}

// NewDHCP4 creates DHCPv4 operator.
func NewDHCP4(logger *zap.Logger, linkName string, routeMetric uint32, platform runtime.Platform) *DHCP4 {
	return &DHCP4{
		logger:      logger,
		linkName:    linkName,
		routeMetric: routeMetric,
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

// Run the operator loop.
//
//nolint:gocyclo,dupl
func (d *DHCP4) Run(ctx context.Context, notifyCh chan<- struct{}) {
	const minRenewDuration = 5 * time.Second // protect from renewing too often

	renewInterval := minRenewDuration

	for {
		leaseTime, err := d.renew(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			d.logger.Warn("renew failed", zap.Error(err), zap.String("link", d.linkName))
		}

		if err == nil {
			select {
			case notifyCh <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}

		if leaseTime > 0 {
			renewInterval = leaseTime / 2
		} else {
			renewInterval /= 2
		}

		if renewInterval < minRenewDuration {
			renewInterval = minRenewDuration
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(renewInterval):
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
func (d *DHCP4) parseAck(ack *dhcpv4.DHCPv4) {
	d.mu.Lock()
	defer d.mu.Unlock()

	addr, _ := netaddr.FromStdIPNet(&net.IPNet{
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
			gw, _ := netaddr.FromStdIP(route.Router)
			dst, _ := netaddr.FromStdIPNet(route.Dest)

			d.routes = append(d.routes, network.RouteSpecSpec{
				Family:      nethelpers.FamilyInet4,
				Destination: dst,
				Source:      addr.IP(),
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
			gw, _ := netaddr.FromStdIP(router)

			d.routes = append(d.routes, network.RouteSpecSpec{
				Family:      nethelpers.FamilyInet4,
				Gateway:     gw,
				Source:      addr.IP(),
				OutLinkName: d.linkName,
				Table:       nethelpers.TableMain,
				Priority:    d.routeMetric,
				Scope:       nethelpers.ScopeGlobal,
				Type:        nethelpers.TypeUnicast,
				Protocol:    nethelpers.ProtocolBoot,
				ConfigLayer: network.ConfigOperator,
			})

			if !addr.Contains(gw) {
				// add an interface route for the gateway if it's not in the same network
				d.routes = append(d.routes, network.RouteSpecSpec{
					Family:      nethelpers.FamilyInet4,
					Destination: netaddr.IPPrefixFrom(gw, gw.BitLen()),
					Source:      addr.IP(),
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

	if len(ack.DNS()) > 0 {
		dns := make([]netaddr.IP, len(ack.DNS()))

		for i := range dns {
			dns[i], _ = netaddr.FromStdIP(ack.DNS()[i])
		}

		d.resolvers = []network.ResolverSpecSpec{
			{
				DNSServers:  dns,
				ConfigLayer: network.ConfigOperator,
			},
		}
	} else {
		d.resolvers = nil
	}

	if ack.HostName() != "" {
		spec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigOperator,
		}

		if err = spec.ParseFQDN(ack.HostName()); err == nil {
			if ack.DomainName() != "" {
				spec.Domainname = ack.DomainName()
			}

			d.hostname = []network.HostnameSpecSpec{
				spec,
			}
		} else {
			d.hostname = nil
		}
	} else {
		d.hostname = nil
	}

	if len(ack.NTPServers()) > 0 {
		ntp := make([]string, len(ack.NTPServers()))

		for i := range ntp {
			ip, _ := netaddr.FromStdIP(ack.NTPServers()[i])
			ntp[i] = ip.String()
		}

		d.timeservers = []network.TimeServerSpecSpec{
			{
				NTPServers:  ntp,
				ConfigLayer: network.ConfigOperator,
			},
		}
	} else {
		d.timeservers = nil
	}
}

func (d *DHCP4) renew(ctx context.Context) (time.Duration, error) {
	opts := []dhcpv4.OptionCode{
		dhcpv4.OptionClasslessStaticRoute,
		dhcpv4.OptionDomainNameServer,
		dhcpv4.OptionDNSDomainSearchList,
		dhcpv4.OptionHostName,
		dhcpv4.OptionNTPServers,
		dhcpv4.OptionDomainName,
	}

	if d.requestMTU {
		opts = append(opts, dhcpv4.OptionInterfaceMTU)
	}

	mods := []dhcpv4.Modifier{dhcpv4.WithRequestedOptions(opts...)}
	clientOpts := []nclient4.ClientOpt{}

	if d.offer != nil {
		// do not use broadcast, but send the packet to DHCP server directly
		addr, err := net.ResolveUDPAddr("udp", d.offer.ServerIPAddr.String()+":67")
		if err != nil {
			return 0, err
		}

		// by default it's set to 0.0.0.0 which actually breaks lease renew
		d.offer.ClientIPAddr = d.offer.YourIPAddr

		clientOpts = append(clientOpts, nclient4.WithServerAddr(addr))
	}

	cli, err := nclient4.New(d.linkName, clientOpts...)
	if err != nil {
		return 0, err
	}

	//nolint:errcheck
	defer cli.Close()

	var lease *nclient4.Lease

	if d.offer != nil {
		lease, err = cli.RequestFromOffer(ctx, d.offer, mods...)
	} else {
		lease, err = cli.Request(ctx, mods...)
	}

	if err != nil {
		// clear offer if request fails to start with discover sequence next time
		d.offer = nil

		return 0, err
	}

	d.logger.Debug("DHCP ACK", zap.String("link", d.linkName), zap.String("dhcp", collapseSummary(lease.ACK.Summary())))

	d.offer = lease.Offer
	d.parseAck(lease.ACK)

	return lease.ACK.IPAddressLeaseTime(time.Minute * 30), nil
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
