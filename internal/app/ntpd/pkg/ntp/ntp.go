/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package ntp

import (
	"errors"
	"log"
	"math/rand"
	"syscall"
	"time"

	"github.com/beevik/ntp"
)

// https://access.redhat.com/solutions/39194
// Using the above as reference for setting min/max
const (
	MaxPoll = 1000
	MinPoll = 20
)

// NTP contains a server address
// and the most recent response from a query
type NTP struct {
	Server   string
	Response *ntp.Response
}

// NewNTPClient instantiates a new ntp client for the
// specified server
func NewNTPClient(server string) *NTP {
	return &NTP{Server: server}
}

// Daemon runs the control loop for query and set time
// We dont ever want the daemon to stop, so we only log
// errors
func (n *NTP) Daemon() (err error) {
	rando := rand.New(rand.NewSource(time.Now().UnixNano()))
	ticker := time.NewTicker(time.Duration(rando.Intn(MaxPoll)+MinPoll) * time.Second)

	// Do an initial hard set of time to ensure clock skew isnt too far off
	var resp *ntp.Response
	resp, err = n.Query()
	if err != nil {
		log.Printf("error querying %s for time, %s", n.Server, err)
		return err
	}
	n.Response = resp

	if err = n.SetTime(resp); err != nil {
		return err
	}

	for {
		<-ticker.C
		// Set some variance with how frequently we poll ntp servers
		resp, err = n.Query()
		if err != nil {
			// As long as we set initial time, we'll treat
			// subsequent errors as nonfatal
			log.Printf("error querying %s for time, %s", n.Server, err)
			continue
		}
		n.Response = resp

		if err = n.SetTime(resp); err != nil {
			log.Printf("failed to set time, %s", err)
			continue
		}
		ticker = time.NewTicker(time.Duration(rando.Intn(MaxPoll)+MinPoll) * time.Second)
	}
}

// Query polls the ntp server to get back a response
// and saves it for later use
func (n *NTP) Query() (*ntp.Response, error) {
	return ntp.Query(n.Server)
}

// SetTime sets the system time based on the query response
func (n *NTP) SetTime(resp *ntp.Response) error {
	// Not sure if this is the right thing to do
	if resp == nil {
		return errors.New("not a valid ntp response")
	}

	timeval := syscall.NsecToTimeval(resp.Time.UnixNano())
	return syscall.Settimeofday(&timeval)
}

// GetTime returns the current system time
func (n *NTP) GetTime() time.Time {
	return time.Now()
}
