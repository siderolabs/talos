// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ntp

import (
	"syscall"
	"time"

	"github.com/beevik/ntp"

	"github.com/talos-systems/talos/internal/pkg/timex"
)

// CurrentTimeFunc provides a function which returns current time.
type CurrentTimeFunc func() time.Time

// QueryFunc provides a function which performs NTP query.
type QueryFunc func(server string) (*ntp.Response, error)

// SetTimeFunc provides a function to set system time.
type SetTimeFunc func(tv *syscall.Timeval) error

// AdjustTimeFunc provides a function to adjust time.
type AdjustTimeFunc func(buf *syscall.Timex) (state timex.State, err error)
