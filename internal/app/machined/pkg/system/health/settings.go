// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package health

import "time"

// Settings configures health check
//
// Fields are similar to k8s pod probe definitions.
type Settings struct {
	InitialDelay time.Duration
	Period       time.Duration
	Timeout      time.Duration

	// StartupWindow enables the startup phase (similar to k8s startup probe), which
	// starts after the InitialDelay.
	//
	// During the startup phase, the health check runs every StartupPeriod with StartupTimeout,
	// failures are not reported (health stays unknown), and the first successful check is reported
	// and ends the startup phase.
	//
	// If the startup window expires without a successful check, the health check switches
	// to the regular settings (Period, Timeout), reporting failures.
	StartupWindow time.Duration
	// StartupPeriod is the health check period during the startup phase (defaults to Period).
	StartupPeriod time.Duration
	// StartupTimeout is the health check timeout during the startup phase (defaults to Timeout).
	StartupTimeout time.Duration
}

// DefaultSettings provides some default health check settings.
var DefaultSettings = Settings{
	InitialDelay: 100 * time.Millisecond,
	Period:       5 * time.Second,
	Timeout:      500 * time.Millisecond,

	StartupWindow:  2 * time.Second,
	StartupPeriod:  200 * time.Millisecond,
	StartupTimeout: 150 * time.Millisecond,
}
