// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"fmt"
	"strings"

	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/reporter"
)

// ConditionReporter is a reporter that reports conditions to a reporter.Reporter.
type ConditionReporter struct {
	w *reporter.Reporter
}

// Update reports a condition to the reporter.
func (r *ConditionReporter) Update(condition conditions.Condition) {
	r.w.Report(conditionToUpdate(condition))
}

// StderrReporter returns console reporter with stderr output.
func StderrReporter() *ConditionReporter {
	return &ConditionReporter{
		w: reporter.New(),
	}
}

func conditionToUpdate(condition conditions.Condition) reporter.Update {
	line := strings.TrimSpace(fmt.Sprintf("waiting for %s", condition.String()))

	switch {
	case strings.HasSuffix(line, "..."):
		return reporter.Update{
			Message: line,
			Status:  reporter.StatusRunning,
		}
	case strings.HasSuffix(line, conditions.OK):
		return reporter.Update{
			Message: line,
			Status:  reporter.StatusSucceeded,
		}
	case strings.HasSuffix(line, conditions.ErrSkipAssertion.Error()):
		return reporter.Update{
			Message: line,
			Status:  reporter.StatusSkip,
		}
	default:
		return reporter.Update{
			Message: line,
			Status:  reporter.StatusError,
		}
	}
}
