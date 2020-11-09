// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ntp

import (
	"fmt"
	"time"
)

// Option allows for the configuration of the ntp client.
type Option func(*NTP) error

const (
	// MaxAllowablePoll is the 'recommended' interval for querying a time server.
	MaxAllowablePoll = 1024
	// MinAllowablePoll is the minimum time allowed for a client to query a time server.
	MinAllowablePoll = 4
	// AdjustTimeLimit is a maximum time drift to compensate via adjtimex().
	AdjustTimeLimit = 128 * time.Millisecond
)

func defaultOptions() *NTP {
	// defaults for minpoll + maxpoll
	// http://www.ntp.org/ntpfaq/NTP-s-algo.htm#AEN2082
	return &NTP{
		Server:  "pool.ntp.org",
		MaxPoll: MaxAllowablePoll * time.Second,
		MinPoll: 64 * time.Second,
	}
}

// WithServer configures the ntp client to use the specified server.
func WithServer(o string) Option {
	return func(n *NTP) (err error) {
		n.Server = o

		return err
	}
}

// WithMaxPoll configures the ntp client MaxPoll interval.
func WithMaxPoll(o int) Option {
	return func(n *NTP) (err error) {
		// TODO: Add in constraints around min/max values from NTP doc.
		if o > MaxAllowablePoll {
			return fmt.Errorf("MaxPoll(%d) is larger than maximum allowed value(%d)", o, MaxAllowablePoll)
		}

		n.MaxPoll = time.Duration(o) * time.Second

		return err
	}
}

// WithMinPoll configures the ntp client MinPoll interval.
func WithMinPoll(o int) Option {
	return func(n *NTP) (err error) {
		if o < MinAllowablePoll {
			return fmt.Errorf("MinPoll(%d) is smaller than minimum allowed value(%d)", o, MinAllowablePoll)
		}

		n.MinPoll = time.Duration(o) * time.Second

		return err
	}
}
