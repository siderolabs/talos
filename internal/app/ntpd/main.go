/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"log"
	"math/rand"
	"syscall"
	"time"

	"github.com/beevik/ntp"
	"github.com/talos-systems/talos/pkg/userdata"
)

// https://access.redhat.com/solutions/39194
// Using the above as reference for setting min/max
const (
	MAXPOLL = 1000
	MINPOLL = 20
	// TODO: Once we get naming sorted we need to apply
	// for a project specific address
	// https://manage.ntppool.org/manage/vendor
	DEFAULTSERVER = "pool.ntp.org"
)

var (
	dataPath *string
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	dataPath = flag.String("userdata", "", "the path to the user data")
	flag.Parse()
}

// New instantiates a new ntp instance against a given server
// If no servers are specified, the default will be used
func main() {
	server := DEFAULTSERVER

	data, err := userdata.Open(*dataPath)
	if err != nil {
		log.Fatalf("open user data: %v", err)
	}

	// Check if ntp servers are defined
	if data.Services.NTPd != nil && data.Services.NTPd.Server != "" {
		server = data.Services.NTPd.Server
	}

	log.Println("Starting ntpd")
	n := &NTP{Server: server}
	n.Daemon()
}

// NTP contains a server address
// and the most recent response from a query
type NTP struct {
	Server   string
	Response *ntp.Response
}

// Daemon runs the control loop for query and set time
// We dont ever want the daemon to stop, so we only log
// errors
func (n *NTP) Daemon() {
	var err error
	rando := rand.New(rand.NewSource(time.Now().UnixNano()))
	ticker := time.NewTicker(time.Duration(rando.Intn(MAXPOLL)+MINPOLL) * time.Second)

	log.Println("initial query")
	// Do an initial hard set of time to ensure clock skew isnt too far off
	if err = n.Query(); err != nil {
		log.Printf("error querying %s for time, %s", n.Server, err)
	}
	log.Printf("%+v\n", n.Response)
	log.Println("Current time")
	log.Println(time.Now())
	if err = n.SetTime(); err != nil {
		log.Printf("failed to set time, %s", err)
	}
	log.Println("Updated time")
	log.Println(time.Now())

	for {
		select {
		case <-ticker.C:
			// Set some variance with how frequently we poll ntp servers
			if err = n.Query(); err != nil {
				log.Printf("error querying %s for time, %s", n.Server, err)
				continue
			}
			log.Printf("%+v\n", n.Response)
			log.Println("Current time")
			log.Println(time.Now())
			if err = n.SetTime(); err != nil {
				log.Printf("failed to set time, %s", err)
				continue
			}
			log.Println("Updated time")
			log.Println(time.Now())
		}
		ticker = time.NewTicker(time.Duration(rando.Intn(MAXPOLL)+MINPOLL) * time.Second)
	}
}

// Query polls the ntp server to get back a response
// and saves it for later use
func (n *NTP) Query() error {
	resp, err := ntp.Query(n.Server)
	if err != nil {
		return err
	}
	n.Response = resp
	return nil
}

// SetTime sets the system time based on the query response
func (n *NTP) SetTime() error {
	// Not sure if this is the right thing to do
	if n.Response == nil {
		return nil
	}
	timeval := syscall.NsecToTimeval(n.Response.Time.UnixNano())
	return syscall.Settimeofday(&timeval)
}
