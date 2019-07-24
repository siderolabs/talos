/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package network

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/kernel"

	"github.com/talos-systems/dhcp/dhcpv4"
	"github.com/talos-systems/dhcp/dhcpv4/client4"
	"github.com/talos-systems/dhcp/netboot"
	"golang.org/x/sys/unix"
)

// DHCPd runs the dhclient process with a certain frequency to maintain a fresh
// dhcp lease
func (service *Service) DHCPd(ctx context.Context, ifname string) {
	var oldLifetime int
	var lifetime int
	var err error
	service.logger.Printf("setting up DHCP on interface %s", ifname)
	for {
		service.logger.Println("obtaining DHCP lease")
		lifetime, err = service.Dhclient(ctx, ifname)
		if err != nil {
			service.logger.Printf("failed to obtain dhcp lease for %s: %+v", ifname, err)

			// Attempt to renew on a shorter interval to not lose network connectivity
			lifetime = oldLifetime / 2
		}
		oldLifetime = lifetime

		select {
		case <-time.After((time.Duration(lifetime / 2)) * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

// Dhclient handles the enture DHCP client interaction from a request to setting
// the received address on the interface
func (service *Service) Dhclient(ctx context.Context, ifname string) (int, error) {
	// TODO: Figure out how we want to pass around ntp servers
	modifiers := []dhcpv4.Modifier{
		dhcpv4.WithRequestedOptions(
			dhcpv4.OptionHostName,
			dhcpv4.OptionClasslessStaticRouteOption,
			dhcpv4.OptionDNSDomainSearchList,
			dhcpv4.OptionNTPServers,
		),
	}

	// Send hostname in Option 12 if we have it
	if hostname, err := os.Hostname(); err != nil {
		modifiers = append(modifiers, dhcpv4.WithOption(dhcpv4.OptHostName(hostname)))
	}

	var err error
	var netconf *netboot.NetConf
	// make dhcp request
	if netconf, err = service.dhclient4(ctx, ifname, modifiers...); err != nil {
		return 0, err
	}

	// verify a single address is returned
	if len(netconf.Addresses) != 1 {
		return 0, fmt.Errorf("expected 1 address in DHCP response for %s, got %d - %+v", ifname, len(netconf.Addresses), netconf.Addresses)
	}

	return netconf.Addresses[0].ValidLifetime, netboot.ConfigureInterface(ifname, netconf)
}

// nolint: gocyclo
func (service *Service) dhclient4(ctx context.Context, ifname string, modifiers ...dhcpv4.Modifier) (*netboot.NetConf, error) {
	attempts := 10
	client := client4.NewClient()
	var (
		conv []*dhcpv4.DHCPv4
		err  error
	)
	for attempt := 0; attempt < attempts; attempt++ {
		service.logger.Printf("requesting DHCP lease: attempt %d of %d", attempt+1, attempts)
		conv, err = client.Exchange(ifname, modifiers...)
		if err != nil && attempt < attempts {
			service.logger.Printf("failed to request DHCP lease: %v", err)
			select {
			case <-time.After(time.Duration(attempt) * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}
		break
	}

	for _, m := range conv {
		if m.OpCode == dhcpv4.OpcodeBootReply && m.MessageType() == dhcpv4.MessageTypeOffer {
			if m.YourIPAddr != nil {
				service.logger.Printf("using IP address %s", m.YourIPAddr.String())
			}

			hostname := m.YourIPAddr.String()
			if m.HostName() != "" {
				hostname = m.HostName()
			}

			// Ignore DHCP-offered hostname if the kernel parameter is set
			var kernHostname *string
			if kernHostname = kernel.Cmdline().Get(constants.KernelParamHostname).First(); kernHostname != nil {
				hostname = *kernHostname
			}

			// Truncate hostname to be betta
			// Allow IP addrs to be valid hostnames for the time being
			if ok := net.ParseIP(hostname); ok == nil {
				// Pull out the first part of a potential FQDN
				hostname = strings.Split(hostname, ".")[0]
			}

			service.logger.Printf("using hostname: %s", hostname)
			if err = unix.Sethostname([]byte(hostname)); err != nil {
				return nil, err
			}

			break
		}
	}

	netconf, _, err := netboot.ConversationToNetconfv4(conv)
	if err != nil {
		return nil, err
	}

	return netconf, err
}
