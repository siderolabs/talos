// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"slices"
	"strings"

	"github.com/siderolabs/gen/maps"
)

// FormatLabels formats labels as a comma-separated key=value pairs.
func FormatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	keys := maps.Keys(labels)
	slices.Sort(keys)

	var sb strings.Builder

	for i, k := range keys {
		if i > 0 {
			sb.WriteString(",")
		}

		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(labels[k])
	}

	return sb.String()
}
