// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"errors"
	"fmt"
	"regexp"
)

// maxNameLength is the maximum length of a hypervisor document name.
// Way lower than NAME_MAX (255), to give us headroom for prefixes/suffixes down the reconciliation path.
const maxNameLength = 63

// validNamePattern matches the characters a hypervisor document name may contain.
var validNamePattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// validateName checks a document name.
func validateName(name string) error {
	switch {
	case name == "":
		return errors.New("name is required")
	case len(name) > maxNameLength:
		return fmt.Errorf("name %q must be %d characters or fewer", name, maxNameLength)
	case !validNamePattern.MatchString(name):
		return fmt.Errorf("name %q: name can only contain ASCII letters, digits and hyphens", name)
	}

	return nil
}
