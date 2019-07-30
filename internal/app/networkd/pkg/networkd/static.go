/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"log"
	"time"

	"github.com/jsimonetti/rtnetlink"
	"golang.org/x/sys/unix"
)

type Static struct {
	NetworkInfo
	Resolv *Resolver
	Route  *Route
}

func (s *Static) Configure(conn *rtnetlink.Conn, idx int) (err error) {
	if err = s.configureAddress(conn, idx); err != nil {
		log.Println("failed addr")
		return err
	}

	if err = s.configureRoutes(conn); err != nil {
		log.Println("failed routes")
		return err
	}

	if err = s.Resolv.Write(); err != nil {
		log.Println("failed resolv write")
		return err
	}

	return err
}

func (s *Static) configureRoutes(conn *rtnetlink.Conn) (err error) {
	if s.Route == nil {
		return err
	}

	exists, err := s.Route.Exists(conn)
	if err != nil {
		return err
	}

	if exists {
		return err
	}

	return s.Route.Add(conn)
}

func (s *Static) configureAddress(conn *rtnetlink.Conn, idx int) (err error) {
	addrInfo := &AddressInfo{
		NetworkInfo: NetworkInfo{
			IP:  s.IP,
			Net: s.Net,
		},
		Scope: unix.RT_SCOPE_UNIVERSE,
		Index: uint32(idx),
	}

	exists, err := addrInfo.Exists(conn)
	if err != nil {
		log.Println("addrinfo.exists error")
		return err
	}

	if exists {
		return err
		/*
			// help: do we really want to drop the address?
			// if the address already exists and is correct,
			// seems like we should just return and call it a day
			//
			// Side benefit to deleting the interface, it cleans up
			// all routes associated with said interface ( or maybe addresses? )
			if err = addrInfo.Delete(conn); err != nil {
				log.Println("addrinfo.delete error")
				return err
			}
		*/
	}

	if err = addrInfo.Add(conn); err != nil {
		log.Println("addrinfo.add error")
		return err
	}

	return err
}

func (s *Static) TTL() time.Duration {
	return 0
}
