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
	"sync"
	"text/template"
	"time"

	"github.com/jsimonetti/rtnetlink"
	"github.com/jsimonetti/rtnetlink/rtnl"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	talosnet "github.com/talos-systems/talos/pkg/net"
	"github.com/talos-systems/talos/pkg/retry"
)

// filterInterfaces filters network links by name so we only manage links
// we need to.
//
// nolint: gocyclo
func filterInterfaces(interfaces []net.Interface) (filtered []net.Interface, err error) {
	var (
		conn     *rtnetlink.Conn
		rtnlConn *rtnl.Conn
	)

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

	// nolint: errcheck
	defer conn.Close()

	rtnlConn = &rtnl.Conn{Conn: conn}

	var wg sync.WaitGroup

	filteredCh := make(chan net.Interface)

	for _, iface := range filtered {
		iface := iface

		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := checkCarrier(conn, rtnlConn, iface); err != nil {
				log.Printf("%s", err)
			} else {
				log.Printf("link %q has carrier signal", iface.Name)

				filteredCh <- iface
			}
		}()
	}

	go func() {
		wg.Wait()
		close(filteredCh)
	}()

	filtered = filtered[:0]

	for iface := range filteredCh {
		filtered = append(filtered, iface)
	}

	return filtered, nil
}

func checkCarrier(conn *rtnetlink.Conn, rtnlConn *rtnl.Conn, iface net.Interface) error {
	link, err := conn.Link.Get(uint32(iface.Index))
	if err != nil {
		return fmt.Errorf("error getting link %q: %w", iface.Name, err)
	}

	if link.Flags&unix.IFF_UP != unix.IFF_UP {
		if err = rtnlConn.LinkUp(&iface); err != nil {
			return fmt.Errorf("error bringing up link %q up: %s", iface.Name, err)
		}

		if err = retry.Constant(10*time.Second, retry.WithUnits(250*time.Millisecond), retry.WithJitter(50*time.Millisecond)).Retry(func() error {
			link, err = conn.Link.Get(uint32(iface.Index))
			if err != nil {
				return retry.UnexpectedError(err)
			}

			if link.Flags&unix.IFF_UP != unix.IFF_UP {
				return retry.ExpectedError(fmt.Errorf("link is not up %s", iface.Name))
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error waiting for link %q to be up: %s", iface.Name, err)
		}
	}

	if link.Attributes.OperationalState != rtnetlink.OperStateUp && link.Attributes.OperationalState != rtnetlink.OperStateUnknown {
		return fmt.Errorf("no carrier for link %q", iface.Name)
	}

	return nil
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

	if domain, err := talosnet.DomainName(); err == nil {
		if domain != "" {
			if _, err = resolvconf.WriteString(fmt.Sprintf("search %s\n", domain)); err != nil {
				return fmt.Errorf("failed to add domain search line to resolvconf: %s", err)
			}
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

{{ with .ExtraHosts }}
{{ range . }}
{{ .IP }} {{ range .Aliases }}{{.}} {{ end }}
{{ end }}
{{ end }}
`

func writeHosts(hostname string, address net.IP, config runtime.Configurator) (err error) {
	extraHosts := []runtime.ExtraHost{}

	if config != nil {
		extraHosts = config.Machine().Network().ExtraHosts()
	}

	data := struct {
		IP         string
		Hostname   string
		Alias      string
		ExtraHosts []runtime.ExtraHost
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

	return ioutil.WriteFile("/etc/hosts", writer.Bytes(), 0644)
}
