// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package quirks contains the quirks for Talos image generation.
package quirks

import "github.com/blang/semver/v4"

// Quirks contains the quirks for Talos image generation.
type Quirks struct {
	v *semver.Version
}

// New returns a new Quirks instance based on Talos version for the image.
func New(talosVersion string) Quirks {
	v, err := semver.ParseTolerant(talosVersion) // ignore the error
	if err != nil {
		return Quirks{}
	}

	return Quirks{v: &v}
}

var minVersionResetOption = semver.MustParse("1.4.0")

// SupportsResetGRUBOption returns true if the Talos version supports the reset option in GRUB menu (image and ISO).
func (q Quirks) SupportsResetGRUBOption() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return true
	}

	return q.v.GTE(minVersionResetOption)
}

var minVersionCompressedMETA = semver.MustParse("1.6.3")

// SupportsCompressedEncodedMETA returns true if the Talos version supports compressed and encoded META as an environment variable.
func (q Quirks) SupportsCompressedEncodedMETA() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return true
	}

	return q.v.GTE(minVersionCompressedMETA)
}
