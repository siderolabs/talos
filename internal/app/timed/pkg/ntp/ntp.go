// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ntp

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/beevik/ntp"
	"github.com/hashicorp/go-multierror"
	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/internal/app/timed/pkg/timex"
)

// NTP contains a server address.
type NTP struct {
	Server  string
	MinPoll time.Duration
	MaxPoll time.Duration

	ready uint32
}

// NewNTPClient instantiates a new ntp client for the
// specified server.
func NewNTPClient(opts ...Option) (*NTP, error) {
	ntp := defaultOptions()

	var result *multierror.Error
	for _, setter := range opts {
		result = multierror.Append(setter(ntp))
	}

	return ntp, result.ErrorOrNil()
}

// Ready checks whether initial time sync has already happened.
func (n *NTP) Ready() bool {
	return atomic.LoadUint32(&n.ready) > 0
}

// Daemon runs the control loop for query and set time
// We dont ever want the daemon to stop, so we only log
// errors.
func (n *NTP) Daemon() (err error) {
	if err = n.QueryAndSetTime(); err != nil {
		log.Println(err)

		// if initial time sync fails, restart the service for more aggressive retry
		return err
	}

	for {
		// Set some variance with how frequently we poll ntp servers.
		// This is based on rand(MaxPoll) + MinPoll so we wait at least
		// MinPoll.
		randSleep := time.Duration(rand.Intn(int(n.MaxPoll.Seconds()))) * time.Second
		time.Sleep(randSleep + n.MinPoll)

		if err = n.QueryAndSetTime(); err != nil {
			log.Println(err)
		}
	}
}

// Query polls the ntp server and verifies a successful response.
func (n *NTP) Query() (resp *ntp.Response, err error) {
	err = retry.Constant(n.MaxPoll, retry.WithUnits(n.MinPoll), retry.WithJitter(250*time.Millisecond)).Retry(func() error {
		resp, err = ntp.Query(n.Server)
		if err != nil {
			log.Printf("query error: %v", err)
			return retry.ExpectedError(err)
		}

		if err = resp.Validate(); err != nil {
			return retry.UnexpectedError(err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query NTP server: %w", err)
	}

	return resp, nil
}

// GetTime returns the current system time.
func (n *NTP) GetTime() time.Time {
	return time.Now()
}

// QueryAndSetTime queries the NTP server and sets the time.
func (n *NTP) QueryAndSetTime() (err error) {
	var resp *ntp.Response

	if resp, err = n.Query(); err != nil {
		return fmt.Errorf("error querying %s for time, %s", n.Server, err)
	}

	if err = adjustTime(resp.ClockOffset); err != nil {
		return fmt.Errorf("failed to set time, %s", err)
	}

	atomic.StoreUint32(&n.ready, 1)

	return
}

// SetTime sets the system time based on the query response.
func setTime(adjustedTime time.Time) error {
	log.Printf("setting time to %s", adjustedTime)

	timeval := syscall.NsecToTimeval(adjustedTime.UnixNano())

	return syscall.Settimeofday(&timeval)
}

// adjustTime adds an offset to the current time.
func adjustTime(offset time.Duration) error {
	if offset < -AdjustTimeLimit || offset > AdjustTimeLimit {
		return setTime(time.Now().Add(offset))
	}

	var buf bytes.Buffer

	fmt.Fprintf(&buf, "adjusting time by %s", offset)

	req := syscall.Timex{
		Modes:  timex.ADJ_OFFSET | timex.ADJ_NANO | timex.ADJ_STATUS,
		Offset: int64(offset / time.Nanosecond),
		Status: timex.STA_PLL,
	}

	state, err := timex.Adjtimex(&req)

	fmt.Fprintf(&buf, ", state %s, status %s", state, timex.Status(req.Status))

	if err != nil {
		fmt.Println(&buf, ", error was %s", err)
	}

	log.Println(buf.String())

	return err
}
