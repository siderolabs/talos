/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/jsimonetti/rtnetlink"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/pkg/userdata"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
}

func main() {

	// Load up userdata
	ud, err := userdata.Open("/var/userdata.yaml")
	if err != nil {
		log.Printf("failed to read userdata %s, using defaults: %+v", "/var/userdata.yaml", err)
	}

	// Handle netlink connection
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		log.Fatal(err)
	}

	// Discover local interfaces
	ints, err := net.Interfaces()
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	for _, netif := range ints {
		var opts []networkd.Option

		opts = append(opts, networkd.WithName(netif.Name))

		// Filter out interfaces that we dont care about
		// Probably(?) a better way to do this
		switch {
		case strings.HasPrefix(netif.Name, "en"):
		case strings.HasPrefix(netif.Name, "eth"):
		//case strings.HasPrefix(netif.Name, "bond"):
		//	opts = append(opts, networkd.WithType(networkd.Bond))
		case strings.HasPrefix(netif.Name, "lo"):
		default:
			log.Printf("skipping %s", netif.Name)
			continue
		}

		// Merge with userdata
		opts = append(opts, userDataLookup(ud, netif.Name, netif.Index)...)

		// Only configure loopback interface if we explicitly need to
		if len(opts) == 1 && strings.HasPrefix(netif.Name, "lo") {
			log.Printf("skipping %s", netif.Name)
			continue
		}

		// Create interface representation
		iface, err := networkd.CreateInterface(opts...)

		log.Printf("configuring %s\n", netif.Name)

		wg.Add(1)
		go func() {
			defer wg.Done()

			// Configure interface
			if err = iface.Setup(conn); err != nil {
				log.Println(err)
				return
			}

			var ttl time.Duration
			for {
				ttl = iface.Addressing.TTL()

				// TTL of 0 should not be renewed
				if ttl == 0 {
					break
				}

				select {
				case <-time.After(ttl / 2):
					// Reconfigure interface ( should be mostly noop )
					// since we're expecting to get the same dhcp response back
					if err = iface.Addressing.Configure(conn, netif.Index); err != nil {
						log.Println(err)
					}
				}

			}
		}()
	}

	wg.Wait()
}

// userDataLookup generates configuration options for the specified interface
// based on the supplied userdata
func userDataLookup(data *userdata.UserData, ifname string, idx int) []networkd.Option {

	// Skip out on any custom network configuration if
	// not specified
	if !validNetworkUserData(data) {
		return nil
	}

	var opts []networkd.Option

	for _, device := range data.Networking.OS.Devices {
		if device.Interface != ifname {
			continue
		}

		// Configure static addressing
		if device.CIDR != "" {
			ip, ipnet, err := net.ParseCIDR(device.CIDR)
			if err != nil {
				log.Printf("invalid CIDR(%s) for %s, skipping: %+v", device.CIDR, ifname, err)
				continue
			}

			s := &networkd.Static{
				NetworkInfo: networkd.NetworkInfo{
					IP:  ip,
					Net: ipnet,
				},
			}
			opts = append(opts, networkd.WithAddressing(s))
		}

		// Configure additional routes
		if len(device.Routes) > 0 {
			for _, route := range device.Routes {
				dip, dipnet, err := net.ParseCIDR(route.Network)
				if err != nil {
					log.Printf("invalid CIDR(%s) for %s, skipping: %+v", route.Network, ifname, err)
					continue
				}
				gip, gipnet, err := net.ParseCIDR(route.Gateway)
				if err != nil {
					log.Printf("invalid CIDR(%s) for %s, skipping: %+v", route.Gateway, ifname, err)
					continue
				}
				r := networkd.Route{
					Dst: &networkd.NetworkInfo{
						IP:  dip,
						Net: dipnet,
					},
					Gateway: &networkd.NetworkInfo{
						IP:  gip,
						Net: gipnet,
					},
					Index: idx,
				}
				opts = append(opts, networkd.WithRoute(r))
			}
		}

		// Configure MTU
		if device.MTU != 0 {
			opts = append(opts, networkd.WithMTU(device.MTU))
		}
	}

	return opts
}

// validateNetworkUserData ensures that we have actual data to do our lookups
func validNetworkUserData(data *userdata.UserData) bool {
	if data == nil {
		return false
	}

	if data.Networking == nil {
		return false
	}

	if data.Networking.OS == nil {
		return false
	}

	if data.Networking.OS.Devices == nil {
		return false
	}

	return true
}
