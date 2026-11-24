// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain

import (
	"errors"
	"fmt"
	"regexp"
)

// aliasPrefix marks a configured device; libvirt keeps "ua-" aliases.
const aliasPrefix = "ua-talos-"

// DeviceAlias names a configured device. name must pass ValidateAliasName.
func DeviceAlias(kind, name string) string {
	return aliasPrefix + kind + "-" + name
}

// aliasNamePattern is what QEMU accepts in a device id after the prefix supplies its leading
// letter. It also leaves out "/" and "@", which separate a disk name in its status ID.
var aliasNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidateAliasName checks that name can be passed to DeviceAlias.
func ValidateAliasName(name string) error {
	switch {
	case name == "":
		return errors.New("name is required")
	case !aliasNamePattern.MatchString(name):
		return fmt.Errorf("name %q can only contain ASCII letters, digits, dots, underscores and hyphens", name)
	}

	return nil
}
