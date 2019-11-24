// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package networkd

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strings"
)

// filterInterfaceByName filters network links by name so we only mange links
// we need to.
func filterInterfaceByName(links []net.Interface) (filteredLinks []net.Interface) {
	for _, link := range links {
		switch {
		case strings.HasPrefix(link.Name, "en"):
			filteredLinks = append(filteredLinks, link)
		case strings.HasPrefix(link.Name, "eth"):
			filteredLinks = append(filteredLinks, link)
		case strings.HasPrefix(link.Name, "lo"):
			filteredLinks = append(filteredLinks, link)
		case strings.HasPrefix(link.Name, "bond"):
			filteredLinks = append(filteredLinks, link)
		}
	}

	return filteredLinks
}

// writeResolvConf generates a /etc/resolv.conf with the specified nameservers.
func writeResolvConf(resolvers []string) error {
	var (
		resolvconf strings.Builder
		err        error
	)

	for idx, resolver := range resolvers {
		// Only allow the first 3 nameservers since that is all that will be used
		if idx >= 3 {
			break
		}

		if _, err = resolvconf.WriteString(fmt.Sprintf("nameserver %s\n", resolver)); err != nil {
			log.Println("failed to add some resolver to resolvconf:", resolver)
			return err
		}
	}

	log.Println("writing resolvconf")

	return ioutil.WriteFile("/etc/resolv.conf", []byte(resolvconf.String()), 0644)
}
