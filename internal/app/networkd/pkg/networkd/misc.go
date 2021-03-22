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

	"github.com/jsimonetti/rtnetlink"
	talosnet "github.com/talos-systems/net"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/machinery/config"
)

// filterInterfaces filters network links by name so we only mange links
// we need to.
//
//nolint:gocyclo
func filterInterfaces(logger *log.Logger, interfaces []net.Interface) (filtered []net.Interface, err error) {
	var conn *rtnetlink.Conn

	for _, iface := range interfaces {
		switch {
		case strings.HasPrefix(iface.Name, "en"):
			filtered = append(filtered, iface)
		case strings.HasPrefix(iface.Name, "eth"):
			filtered = append(filtered, iface)
		case strings.HasPrefix(iface.Name, "lo"):
			filtered = append(filtered, iface)
		case strings.HasPrefix(iface.Name, "bond"):
			filtered = append(filtered, iface)
		}
	}

	conn, err = rtnetlink.Dial(nil)
	if err != nil {
		return nil, err
	}

	//nolint:errcheck
	defer conn.Close()

	n := 0 //nolint:wsl
	for _, iface := range filtered {
		link, err := conn.Link.Get(uint32(iface.Index))
		if err != nil {
			logger.Printf("error getting link %q", iface.Name)

			continue
		}

		if link.Flags&unix.IFF_UP == unix.IFF_UP && !(link.Flags&unix.IFF_RUNNING == unix.IFF_RUNNING) {
			logger.Printf("no carrier for link %q", iface.Name)
		} else {
			logger.Printf("link %q has carrier signal", iface.Name)
			filtered[n] = iface
			n++
		}
	}

	filtered = filtered[:n]

	return filtered, nil
}

// writeResolvConf generates a /etc/resolv.conf with the specified nameservers.
func writeResolvConf(logger *log.Logger, resolvers []string) (err error) {
	var resolvconf strings.Builder

	for idx, resolver := range resolvers {
		// Only allow the first 3 nameservers since that is all that will be used
		if idx >= 3 {
			break
		}

		if _, err = resolvconf.WriteString(fmt.Sprintf("nameserver %s\n", resolver)); err != nil {
			logger.Println("failed to add some resolver to resolvconf:", resolver)

			return err
		}
	}

	if domain, err := talosnet.DomainName(); err == nil {
		if domain != "" {
			if _, err = resolvconf.WriteString(fmt.Sprintf("search %s\n", domain)); err != nil {
				return fmt.Errorf("failed to add domain search line to resolvconf: %s", err)
			}
		}
	}

	logger.Println("writing resolvconf")

	return ioutil.WriteFile("/etc/resolv.conf", []byte(resolvconf.String()), 0o644)
}

const hostsTemplate = `
127.0.0.1       localhost
{{ .IP }}       {{ .Hostname }} {{ if ne .Hostname .Alias }}{{ .Alias }}{{ end }}
::1             localhost ip6-localhost ip6-loopback
ff02::1         ip6-allnodes
ff02::2         ip6-allrouters

{{ with .ExtraHosts }}
{{ range . }}
{{ .IP }} {{ range .Aliases }}{{.}} {{ end }}
{{ end }}
{{ end }}
`

func writeHosts(hostname string, address net.IP, cfg config.Provider) (err error) {
	extraHosts := []config.ExtraHost{}

	if cfg != nil {
		extraHosts = cfg.Machine().Network().ExtraHosts()
	}

	data := struct {
		IP         string
		Hostname   string
		Alias      string
		ExtraHosts []config.ExtraHost
	}{
		IP:         address.String(),
		Hostname:   hostname,
		Alias:      strings.Split(hostname, ".")[0],
		ExtraHosts: extraHosts,
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

	return ioutil.WriteFile("/etc/hosts", writer.Bytes(), 0o644)
}
