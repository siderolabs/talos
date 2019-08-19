/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strings"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/pkg/userdata"
)

// filterInterfaceByName filters network links by name so we only mange links
// we need to
func filterInterfaceByName(links []*net.Interface) (filteredLinks []*net.Interface) {
	for _, link := range links {
		switch {
		case strings.HasPrefix(link.Name, "en"):
			filteredLinks = append(filteredLinks, link)
		case strings.HasPrefix(link.Name, "eth"):
			filteredLinks = append(filteredLinks, link)
		case strings.HasPrefix(link.Name, "lo"):
			filteredLinks = append(filteredLinks, link)
		}
	}

	return filteredLinks
}

// parseLinkMessage creates the base set of attributes for nic creation
func parseLinkMessage(link *net.Interface) []nic.Option {
	opts := []nic.Option{}

	opts = append(opts, nic.WithName(link.Name))
	opts = append(opts, nic.WithMTU(uint32(link.MTU)))
	opts = append(opts, nic.WithIndex(uint32(link.Index)))

	// Ensure lo has proper loopback address
	// Ensure MTU for loopback is 64k
	// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=0cf833aefaa85bbfce3ff70485e5534e09254773
	if strings.HasPrefix(link.Name, "lo") {
		opts = append(opts, nic.WithAddressing(
			&address.Static{
				Device: &userdata.Device{
					CIDR: "127.0.0.1/8",
					MTU:  65536,
				},
				NetIf: link,
			},
		))
	}

	return opts
}

// writeResolvConf generates a /etc/resolv.conf with the specified nameservers.
func writeResolvConf(resolvers []net.IP) error {
	if len(resolvers) == 0 {
		log.Printf("no DNS servers defined, using defaults %s and %s\n", DefaultPrimaryResolver, DefaultSecondaryResolver)
		resolvers = []net.IP{net.ParseIP(DefaultPrimaryResolver), net.ParseIP(DefaultSecondaryResolver)}
	}

	var resolvconf strings.Builder
	var err error
	for idx, resolver := range resolvers {
		// Only allow the first 3 nameservers since that is all that will be used
		if idx >= 3 {
			break
		}
		if _, err = resolvconf.WriteString(fmt.Sprintf("nameserver %s\n", resolver)); err != nil {
			log.Println("failed to add some resolver to resolvconf")
			return err
		}
	}

	log.Println("writing resolvconf")
	return ioutil.WriteFile("/etc/resolv.conf", []byte(resolvconf.String()), 0644)
}
