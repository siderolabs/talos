/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package network

import (
	"log"
	"time"

	"github.com/talos-systems/dhcp/dhcpv4"
	"github.com/talos-systems/dhcp/dhcpv4/client4"
	"github.com/talos-systems/dhcp/netboot"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// Setup creates the network.
func Setup(platform string) (err error) {

	// TODO: Turn this into a log level
	/*
		log.Println("All available network links")
		links, _ := netlink.LinkList()
		for _, link := range links {
			log.Printf("%+v", link)
		}
	*/

	//ifup lo
	ifname := "lo"
	link, err := netlink.LinkByName(ifname)
	if err != nil {
		return err
	}
	if err = netlink.LinkSetUp(link); err != nil {
		return err
	}

	//ifup eth0
	ifname = "eth0"
	link, err = netlink.LinkByName(ifname)
	if err != nil {
		return err
	}
	if err = netlink.LinkSetUp(link); err != nil {
		return err
	}

	//dhcp request
	// TODO: Figure out how we want to pass around ntp servers
	modifiers := []dhcpv4.Modifier{
		dhcpv4.WithRequestedOptions(
			dhcpv4.OptionHostName,
			dhcpv4.OptionClasslessStaticRouteOption,
			dhcpv4.OptionDNSDomainSearchList,
			dhcpv4.OptionNTPServers,
		),
	}
	if err = dhclient(ifname, modifiers); err != nil {
		return err
	}

	// Set up dhcp renewals every 5m
	go func() {
		for {
			// TODO pick this out of the dhclient/netconf response
			// so we can request less frequently
			time.Sleep(5 * time.Minute)
			log.Println("Renewing dhcp lease")
			if err = dhclient(ifname, modifiers); err != nil {
				// Probably need to do something better here but not sure there's much to do
				log.Println("Failed to renew dhcp lease, ", err)
			}
		}
	}()
	return nil
}

func dhclient(ifname string, modifiers []dhcpv4.Modifier) error {
	var err error
	var netconf *netboot.NetConf

	if netconf, err = dhclient4(ifname, modifiers...); err != nil {
		return err
	}
	if err = netboot.ConfigureInterface(ifname, netconf); err != nil {
		return err
	}

	return err
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
