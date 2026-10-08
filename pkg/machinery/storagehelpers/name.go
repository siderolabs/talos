// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storagehelpers

import (
	"errors"
	"fmt"
	"regexp"
)

// MaxStoragePoolNameLength is the maximum length of a storage pool name.
//
// Well below NAME_MAX: a pool's name is a directory under its backing volume's mount, and the names
// of the volumes within it are built from names bounded separately.
const MaxStoragePoolNameLength = 63

// storagePoolNamePattern matches the characters a storage pool name may contain.
//
// Underscores are allowed within the name but not at its start, so a pool name is always
// distinguishable from the staging and reserved prefixes which use one.
var storagePoolNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`)

// ValidateStoragePoolName checks a storage pool name: nonempty, ASCII letters, digits, hyphens or
// underscores, starting with a letter or digit, and at most MaxStoragePoolNameLength characters.
func ValidateStoragePoolName(name string) error {
	switch {
	case name == "":
		return errors.New("storage pool name is required")
	case len(name) > MaxStoragePoolNameLength:
		return fmt.Errorf("storage pool name %q must be %d characters or fewer", name, MaxStoragePoolNameLength)
	case !storagePoolNamePattern.MatchString(name):
		return fmt.Errorf(
			"storage pool name %q: name can only contain ASCII letters, digits, hyphens and underscores, and must start with a letter or digit",
			name)
	}

	return nil
}

// ValidStoragePoolName reports whether name is one a storage pool may be called.
//
// It exists for callers that reference a pool by name and phrase their own message naming the kind
// referenced, rather than reporting a name of their own.
func ValidStoragePoolName(name string) bool {
	return storagePoolNamePattern.MatchString(name)
}
