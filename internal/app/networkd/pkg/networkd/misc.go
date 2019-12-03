// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package networkd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"text/template"
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
func writeResolvConf(resolvers []string) (err error) {
	var resolvconf strings.Builder

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

const hostsTemplate = `
127.0.0.1       localhost
{{ .IP }}       {{ .Hostname }} {{ if ne .Hostname .Alias }}{{ .Alias }}{{ end }}
::1             localhost ip6-localhost ip6-loopback
ff02::1         ip6-allnodes
ff02::2         ip6-allrouters
`

func writeHosts(hostname string, address net.IP) (err error) {
	data := struct {
		IP       string
		Hostname string
		Alias    string
	}{
		IP:       address.String(),
		Hostname: hostname,
		Alias:    strings.Split(hostname, ".")[0],
	}

	var tmpl *template.Template

	tmpl, err = template.New("").Parse(hostsTemplate)
	if err != nil {
		return err
	}

	var buf []byte

	writer := bytes.NewBuffer(buf)

	err = tmpl.Execute(writer, data)
	if err != nil {
		return err
	}

	return ioutil.WriteFile("/etc/hosts", writer.Bytes(), 0644)
}
