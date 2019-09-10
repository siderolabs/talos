/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package ntp

import (
	"fmt"
	"log"
	"math/rand"
	"syscall"
	"time"

	"github.com/beevik/ntp"
	"github.com/hashicorp/go-multierror"
)

// NTP contains a server address
type NTP struct {
	Server  string
	MinPoll time.Duration
	MaxPoll time.Duration
	Retry   int
}

// NewNTPClient instantiates a new ntp client for the
// specified server
func NewNTPClient(opts ...Option) (*NTP, error) {
	ntp := defaultOptions()

	var result *multierror.Error
	for _, setter := range opts {
		result = multierror.Append(setter(ntp))
	}

	return ntp, result.ErrorOrNil()
}

// Daemon runs the control loop for query and set time
// We dont ever want the daemon to stop, so we only log
// errors
func (n *NTP) Daemon() (err error) {
	// Do an initial hard set of time to ensure clock skew isnt too far off
	var resp *ntp.Response
	if resp, err = n.Query(); err != nil {
		log.Printf("error querying %s for time, %s", n.Server, err)
		return err
	}

	if err = setTime(resp.Time); err != nil {
		return err
	}

	var randSleep time.Duration
	for {
		// Set some variance with how frequently we poll ntp servers.
		// This is based on rand(MaxPoll) + MinPoll so we wait at least
		// MinPoll.
		randSleep = time.Duration(rand.Intn(int(n.MaxPoll.Seconds()))) * time.Second
		time.Sleep(randSleep + n.MinPoll)

		if resp, err = n.Query(); err != nil {
			// As long as we set initial time, we'll treat
			// subsequent errors as nonfatal
			log.Printf("error querying %s for time, %s", n.Server, err)
			continue
		}

		if err = adjustTime(resp.ClockOffset); err != nil {
			log.Printf("failed to set time, %s", err)
			continue
		}
	}
}

// Query polls the ntp server and verifies a successful response.
func (n *NTP) Query() (*ntp.Response, error) {
	for i := 0; i < n.Retry; i++ {
		resp, err := ntp.Query(n.Server)
		if err != nil {
			time.Sleep(time.Duration(i) * n.MinPoll)
			continue
		}

		if err := resp.Validate(); err != nil {
			time.Sleep(time.Duration(i) * n.MinPoll)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("failed to get a response back from ntp server after %d retries", n.Retry)
}

// GetTime returns the current system time
func (n *NTP) GetTime() time.Time {
	return time.Now()
}

// SetTime sets the system time based on the query response
func setTime(adjustedTime time.Time) error {
	log.Printf("setting time to %s", adjustedTime)

	timeval := syscall.NsecToTimeval(adjustedTime.UnixNano())
	return syscall.Settimeofday(&timeval)
}

// adjustTime adds an offset to the current time
func adjustTime(offset time.Duration) error {
	return setTime(time.Now().Add(offset))
}
