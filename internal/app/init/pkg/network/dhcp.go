/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package network

import (
	"fmt"
	"log"
	"time"

	"github.com/talos-systems/dhcp/dhcpv4"
	"github.com/talos-systems/dhcp/dhcpv4/client4"
	"github.com/talos-systems/dhcp/netboot"
	"golang.org/x/sys/unix"
)

// DHCPd runs the dhclient process with a certain frequency to maintain a fresh
// dhcp lease
func DHCPd(ifname string) {
	go func() {
		var oldLifetime int
		var lifetime int
		var err error
		log.Printf("setting up DHCP on interface %s", ifname)
		for {
			log.Println("obtaining DHCP lease")
			lifetime, err = Dhclient(ifname)
			if err != nil {
				log.Printf("failed to obtain dhcp lease for %s: %+v", ifname, err)

				// Attempt to renew on a shorter interval to not lose network connectivity
				lifetime = oldLifetime / 2
			}
			oldLifetime = lifetime
			time.Sleep((time.Duration(lifetime / 2)) * time.Second)
		}
	}()
}

// Dhclient handles the enture DHCP client interaction from a request to setting
// the received address on the interface
func Dhclient(ifname string) (int, error) {
	// TODO: Figure out how we want to pass around ntp servers
	modifiers := []dhcpv4.Modifier{
		dhcpv4.WithRequestedOptions(
			dhcpv4.OptionHostName,
			dhcpv4.OptionClasslessStaticRouteOption,
			dhcpv4.OptionDNSDomainSearchList,
			dhcpv4.OptionNTPServers,
		),
	}

	var err error
	var netconf *netboot.NetConf
	// make dhcp request
	if netconf, err = dhclient4(ifname, modifiers...); err != nil {
		return 0, err
	}

	// verify a single address is returned
	if len(netconf.Addresses) != 1 {
		return 0, fmt.Errorf("expected 1 address in DHCP response for %s, got %d - %+v", ifname, len(netconf.Addresses), netconf.Addresses)
	}

	return netconf.Addresses[0].ValidLifetime, netboot.ConfigureInterface(ifname, netconf)
}

// nolint: gocyclo
func dhclient4(ifname string, modifiers ...dhcpv4.Modifier) (*netboot.NetConf, error) {
	attempts := 10
	client := client4.NewClient()
	var (
		conv []*dhcpv4.DHCPv4
		err  error
	)
	for attempt := 0; attempt < attempts; attempt++ {
		log.Printf("requesting DHCP lease: attempt %d of %d", attempt+1, attempts)
		conv, err = client.Exchange(ifname, modifiers...)
		if err != nil && attempt < attempts {
			log.Printf("failed to request DHCP lease: %v", err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		break
	}

	for _, m := range conv {
		if m.OpCode == dhcpv4.OpcodeBootReply && m.MessageType() == dhcpv4.MessageTypeOffer {
			if m.YourIPAddr != nil {
				log.Printf("using IP address %s", m.YourIPAddr.String())
			}

			hostname := m.YourIPAddr.String()
			if m.HostName() != "" {
				hostname = m.HostName()
			}
			log.Printf("using hostname: %s", hostname)
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
