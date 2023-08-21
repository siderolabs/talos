// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package nodename provides utility functions to generate nodenames.
package nodename

import (
	"fmt"
	"strings"
)

// FromHostname converts a hostname to Kubernetes Node name.
//
// UNIX hostname has almost no restrictions, but Kubernetes Node name has
// to be RFC 1123 compliant. This function converts a hostname to a valid
// Kubernetes Node name (if possible).
//
// The allowed format is:
//
//	[a-z0-9]([-a-z0-9]*[a-z0-9])?
//
//nolint:gocyclo
func FromHostname(hostname string) (string, error) {
	nodename := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			// allow lowercase
			return r
		case r >= 'A' && r <= 'Z':
			// lowercase uppercase letters
			return r - 'A' + 'a'
		case r >= '0' && r <= '9':
			// allow digits
			return r
		case r == '-' || r == '_':
			// allow dash, convert underscore to dash
			return '-'
		case r == '.':
			// allow dot
			return '.'
		default:
			// drop anything else
			return -1
		}
	}, hostname)

	// now drop any dashes/dots at the beginning or end
	nodename = strings.Trim(nodename, "-.")

	if len(nodename) == 0 {
		return "", fmt.Errorf("could not convert hostname %q to a valid Kubernetes Node name", hostname)
	}

	return nodename, nil
}
