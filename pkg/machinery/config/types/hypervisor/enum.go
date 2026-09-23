// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// expectedValues renders the members of an enum for a validation error message.
func expectedValues(values []string) string {
	values = hypervisorhelpers.NameableValues(values)

	if len(values) < 2 {
		return strings.Join(values, "")
	}

	return strings.Join(values[:len(values)-1], ", ") + " or " + values[len(values)-1]
}
