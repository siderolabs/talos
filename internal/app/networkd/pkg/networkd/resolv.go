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
)

type Resolver struct {
	Servers []net.IP
	Search  []string
}

func (r *Resolver) Write() error {
	if len(r.Servers) == 0 {
		log.Printf("no DNS servers in dhcp response")
		return nil
	}

	log.Printf("resolv.write.r: %+v", r)
	var resolvconf strings.Builder
	var err error
	for idx, resolver := range r.Servers {
		if idx >= 3 {
			break
		}
		log.Println(idx, resolver)
		if _, err = resolvconf.WriteString(fmt.Sprintf("nameserver %s\n", resolver)); err != nil {
			log.Println("failde to add some resolver to resolvconf")
			return err
		}
	}

	if _, err = resolvconf.WriteString(fmt.Sprintf("search %s\n", strings.Join(r.Search, " "))); err != nil {
		log.Println("failde to add search string to resolvconf")
		return err
	}

	log.Println("writing resolvconf")
	return ioutil.WriteFile("/etc/resolv.conf", []byte(resolvconf.String()), 0644)
}
