/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"log"
	"net"

	"github.com/jsimonetti/rtnetlink"
	"golang.org/x/sys/unix"
)

type Route struct {
	Src      *NetworkInfo
	Dst      *NetworkInfo
	Gateway  *NetworkInfo
	Family   uint8
	Scope    uint8
	Protocol uint8
	Index    int
}

func (r *Route) Add(conn *rtnetlink.Conn) (err error) {
	routeMsg := r.Message()

	log.Printf("route %+v", routeMsg)
	log.Printf("route attrs %+v", routeMsg.Attributes)

	return conn.Route.Add(routeMsg)
}

func (r *Route) Delete(conn *rtnetlink.Conn) (err error) {
	routeMsg := r.Message()

	log.Printf("del route msg %+v", routeMsg)
	log.Printf("del route attrs %+v", routeMsg.Attributes)

	return conn.Route.Delete(routeMsg)
}

func (r *Route) Message() *rtnetlink.RouteMessage {
	attr := rtnetlink.RouteAttributes{}

	routeMsg := &rtnetlink.RouteMessage{
		Family:   r.Family,
		Table:    unix.RT_TABLE_MAIN,
		Protocol: r.Protocol,
		Scope:    r.Scope,
		Type:     unix.RTN_UNICAST,
	}

	if r.Src != nil {
		attr.Src = r.Src.IP
		ones, _ := r.Src.Net.Mask.Size()
		routeMsg.SrcLength = uint8(ones)
	}

	if r.Dst != nil {
		attr.Dst = r.Dst.IP
		ones, _ := r.Dst.Net.Mask.Size()
		routeMsg.DstLength = uint8(ones)
	}

	if r.Gateway != nil {
		attr.Gateway = r.Gateway.IP
		if r.Gateway.IP.Equal(net.IPv4zero) {
			routeMsg.DstLength = uint8(0)
		}
	}

	attr.OutIface = uint32(r.Index)

	routeMsg.Attributes = attr

	return routeMsg
}

func (r *Route) Exists(conn *rtnetlink.Conn) (bool, error) {
	rl, err := conn.Route.List()
	if err != nil {
		return false, err
	}

	log.Printf("%+v", r)
	log.Printf("%+v", r.Src)
	log.Printf("%+v", r.Dst)
	log.Printf("%+v", r.Gateway)
	for _, route := range rl {
		if r.Index != int(route.Attributes.OutIface) {
			continue
		}

		log.Printf("%+v", route)
		// This feels super ugly
		// Only compare against what was given
		if r.Dst != nil {
			if !compareNets(r.Dst.IP, route.Attributes.Dst) {
				continue
			}
		}

		if r.Src != nil {
			if !compareNets(r.Src.IP, route.Attributes.Src) {
				continue
			}
		}

		if r.Gateway != nil {
			if !compareNets(r.Gateway.IP, route.Attributes.Gateway) {
				continue
			}
		}

		// TODO see if we can compare dstlength, srclength, scope
		return true, err

	}

	return false, err
}

func compareNets(a, b net.IP) bool {
	if a == nil && b == nil {
		return true
	}

	if a != nil && a.Equal(b) {
		return true
	}

	return false
}
