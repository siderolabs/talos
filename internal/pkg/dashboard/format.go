// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"cmp"
	"fmt"
	"math"
	"strings"
	"time"
)

// Formatting and ordering helpers shared by the screens built from COSI resources.

// compareMetricDesc orders two metrics from the largest down, keeping unknown (NaN) values last:
// an entry the node reported no metric for is not the quietest one, it is one we know nothing
// about, and it belongs at the bottom either way.
func compareMetricDesc(a, b float64) int {
	aKnown, bKnown := !math.IsNaN(a), !math.IsNaN(b)

	switch {
	case !aKnown && !bKnown:
		return 0
	case !aKnown:
		return 1
	case !bKnown:
		return -1
	default:
		return cmp.Compare(b, a)
	}
}

// colored wraps text in a tview color tag.
func colored(color, text string) string {
	return "[" + color + "]" + text + "[-]"
}

// metricUnknown is displayed in place of a metric the node did not report.
const metricUnknown = "—"

// renderFields renders label/value pairs with the labels padded to a common width.
func renderFields(fields [][2]string) string {
	width := 0
	for _, field := range fields {
		width = max(width, len(field[0]))
	}

	var sb strings.Builder

	for _, field := range fields {
		fmt.Fprintf(&sb, "[::b]%-*s[::-]  %s\n", width, field[0], field[1])
	}

	return sb.String()
}

// formatTimestamp renders a point in time, or a placeholder when it is unset.
func formatTimestamp(timestamp time.Time) string {
	if timestamp.IsZero() {
		return metricUnknown
	}

	return timestamp.Format(time.DateTime)
}

// none is displayed for an empty list of values.
const none = "<none>"
