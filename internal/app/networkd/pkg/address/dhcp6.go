// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package address

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/nclient6"
	"github.com/jsimonetti/rtnetlink"
	"github.com/talos-systems/go-retry/retry"
	"golang.org/x/sys/unix"
)

// DHCP6 implements the Addressing interface.
type DHCP6 struct {
	Reply *dhcpv6.Message
	NetIf *net.Interface
	Mtu   int
}

// Name returns back the name of the address method.
func (d *DHCP6) Name() string {
	return "dhcp6"
}

// Link returns the underlying net.Interface that this address
// method is configured for.
func (d *DHCP6) Link() *net.Interface {
	return d.NetIf
}

// Discover handles the DHCP client exchange stores the DHCP Ack.
func (d *DHCP6) Discover(ctx context.Context, logger *log.Logger, link *net.Interface) error {
	d.NetIf = link
	err := d.discover(ctx, logger)

	return err
}

// Address returns back the IP address from the received DHCP offer.
func (d *DHCP6) Address() *net.IPNet {
	if d.Reply.Options.OneIANA() == nil {
		return nil
	}

	return &net.IPNet{
		IP:   d.Reply.Options.OneIANA().Options.OneAddress().IPv6Addr,
		Mask: net.CIDRMask(128, 128),
	}
}

// Mask returns the netmask from the DHCP offer.
func (d *DHCP6) Mask() net.IPMask {
	return net.CIDRMask(128, 128)
}

// MTU returs the MTU size from the DHCP offer.
func (d *DHCP6) MTU() uint32 {
	if d.Mtu > 0 {
		return uint32(d.Mtu)
	}

	return uint32(d.NetIf.MTU)
}

// TTL denotes how long a DHCP offer is valid for.
func (d *DHCP6) TTL() time.Duration {
	if d.Reply == nil {
		return 0
	}

	return d.Reply.Options.OneIANA().Options.OneAddress().ValidLifetime
}

// Family qualifies the address as ipv4 or ipv6.
func (d *DHCP6) Family() int {
	return unix.AF_INET6
}

// Scope sets the address scope.
func (d *DHCP6) Scope() uint8 {
	return unix.RT_SCOPE_UNIVERSE
}

// Valid denotes if this address method should be used.
func (d *DHCP6) Valid() bool {
	return d.Reply != nil && d.Reply.Options.OneIANA() != nil
}

// Routes is not supported on IPv6.
func (d *DHCP6) Routes() (routes []*Route) {
	return nil
}

// Resolvers returns the DNS resolvers from the DHCP offer.
func (d *DHCP6) Resolvers() []net.IP {
	return d.Reply.Options.DNS()
}

// Hostname returns the hostname from the DHCP offer.
func (d *DHCP6) Hostname() (hostname string) {
	fqdn := d.Reply.Options.FQDN()

	if fqdn != nil && fqdn.DomainName != nil {
		hostname = strings.Join(fqdn.DomainName.Labels, ".")
	} else {
		hostname = fmt.Sprintf("%s-%s", "talos", strings.ReplaceAll(d.Address().IP.String(), ":", ""))
	}

	return hostname
}

// discover handles the actual DHCP conversation.
func (d *DHCP6) discover(ctx context.Context, logger *log.Logger) error {
	if err := waitIPv6LinkReady(logger, d.NetIf); err != nil {
		logger.Printf("failed waiting for IPv6 readiness: %s", err)

		return err
	}

	cli, err := nclient6.New(d.NetIf.Name)
	if err != nil {
		logger.Printf("failed to create dhcp6 client: %s", err)

		return err
	}

	//nolint:errcheck
	defer cli.Close()

	reply, err := cli.RapidSolicit(ctx)
	if err != nil {
		// TODO: Make this a well defined error so we can make it not fatal
		logger.Printf("failed dhcp6 request for %q: %v", d.NetIf.Name, err)

		return err
	}

	logger.Printf("DHCP6 REPLY on %q: %s", d.NetIf.Name, collapseSummary(reply.Summary()))

	d.Reply = reply

	return nil
}

func waitIPv6LinkReady(logger *log.Logger, iface *net.Interface) error {
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return err
	}

	defer conn.Close() //nolint:errcheck

	return retry.Constant(30*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
		ready, err := isIPv6LinkReady(logger, iface, conn)
		if err != nil {
			return retry.UnexpectedError(err)
		}

		if !ready {
			return retry.ExpectedError(fmt.Errorf("IPv6 address is still tentative"))
		}

		return nil
	})
}

// isIPv6LinkReady returns true if the interface has a link-local address
// which is not tentative.
func isIPv6LinkReady(logger *log.Logger, iface *net.Interface, conn *rtnetlink.Conn) (bool, error) {
	addrs, err := conn.Address.List()
	if err != nil {
		return false, err
	}

	for _, addr := range addrs {
		if addr.Index != uint32(iface.Index) {
			continue
		}

		if addr.Family != unix.AF_INET6 {
			continue
		}

		if addr.Attributes.Address.IsLinkLocalUnicast() && (addr.Flags&unix.IFA_F_TENTATIVE == 0) {
			if addr.Flags&unix.IFA_F_DADFAILED != 0 {
				logger.Printf("DADFAILED for %v, continuing anyhow", addr.Attributes.Address)
			}

			return true, nil
		}
	}

	return false, nil
}
