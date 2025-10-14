// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"
	"net"
	"os"

	"github.com/miekg/dns"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/provision"
)

// DNSd entrypoint.
func DNSd(ips []net.IP, resolvConfPath string) error {
	var eg errgroup.Group

	config, err := dns.ClientConfigFromFile(resolvConfPath)
	if err != nil {
		return fmt.Errorf("failed to read %q: %v", resolvConfPath, err)
	}

	for _, ip := range ips {
		eg.Go(func() error {
			addr := net.JoinHostPort(ip.String(), "53")

			server := &dns.Server{
				Addr:    addr,
				Net:     "udp",
				Handler: dns.HandlerFunc(forwardHandler(config)),
			}

			return server.ListenAndServe()
		})
	}

	return eg.Wait()
}

const (
	dnsPid = "dnsd.pid"
	dnsLog = "dnsd.log"
)

// CreateDNSd creates the DNSd server.
func (p *Provisioner) CreateDNSd(state *State, clusterReq provision.ClusterRequest) error {
	return p.startDNSd(state, clusterReq)
}

// DestroyDNSd destoys DNSd server.
func (p *Provisioner) DestroyDNSd(state *State) error {
	return p.stopDNSd(state)
}

func forwardHandler(config *dns.ClientConfig) func(w dns.ResponseWriter, r *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		c := new(dns.Client)
		c.Net = "udp"

		var (
			resp *dns.Msg
			err  error
		)

		for _, serverAddr := range config.Servers {
			resp, _, err = c.Exchange(r, net.JoinHostPort(serverAddr, config.Port))
			if err == nil {
				break
			}
		}

		if resp == nil {
			dns.HandleFailed(w, r)

			return
		}

		if err := w.WriteMsg(resp); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write response: %v\n", err)
		}
	}
}
