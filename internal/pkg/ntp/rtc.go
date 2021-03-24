// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ntp

import (
	"sync"

	"github.com/u-root/u-root/pkg/rtc"
)

// Global instance of RTC clock because `rtc` doesn't support closing.
var (
	RTCClock           *rtc.RTC
	RTCClockInitialize sync.Once
)
