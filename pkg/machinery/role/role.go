// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package role

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/go-multierror"
)

// Role represents Talos user role.
// Its string value is used everywhere: as the the Organization value of Talos client certificate,
// as the value of talosctl flag, etc.
type Role string

const (
	// Admin defines Talos role for admins.
	Admin = Role("os:admin")

	// Reader defines Talos role for readers who can access read-only APIs that do not expose secrets.
	Reader = Role("os:reader")

	// Impersonator defines internal Talos role for impersonating another user (and their role).
	Impersonator = Role("os:impersonator")
)

// Set represents a set of roles.
type Set map[Role]struct{}

// all roles, including internal ones.
var all = MakeSet(Admin, Reader, Impersonator)

// MakeSet makes a set of roles from constants.
// Use Parse in other cases.
func MakeSet(roles ...Role) Set {
	res := make(Set, len(roles))
	for _, r := range roles {
		res[r] = struct{}{}
	}

	return res
}

// Parse parses a set of roles.
// The returned set is always non-nil and contains all roles, including unknown (for compatibility with future versions).
// The returned error contains roles unknown to the current version.
func Parse(str []string) (Set, error) {
	res := make(Set)

	var err *multierror.Error

	for _, r := range str {
		role := Role(r)
		if _, ok := all[role]; !ok {
			err = multierror.Append(err, fmt.Errorf("unexpected role %q", r))
		}

		role = Role(strings.TrimSpace(r))
		if role == "" {
			continue
		}

		res[role] = struct{}{}
	}

	return res, err.ErrorOrNil()
}

// Strings returns a set as a slice of strings.
func (s Set) Strings() []string {
	res := make([]string, 0, len(s))

	for r := range s {
		res = append(res, string(r))
	}

	sort.Strings(res)

	return res
}

// IncludesAny returns true if there is a non-empty intersection between sets.
func (s Set) IncludesAny(other Set) bool {
	for r := range other {
		if _, ok := s[r]; ok {
			return true
		}
	}

	return false
}
