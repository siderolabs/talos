// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager

import (
	"fmt"

	"github.com/siderolabs/talos/pkg/reporter"
)

// progressPrintf wraps a reporter.Reporter to report progress via Printf logging.
func progressPrintf(report *reporter.Reporter, status reporter.Update) func(format string, args ...any) {
	return func(format string, args ...any) {
		msg := status.Message
		extra := fmt.Sprintf(format, args...)

		if extra != "" {
			msg += "\n\t" + extra
		}

		report.Report(reporter.Update{
			Message: msg,
			Status:  status.Status,
		})
	}
}
