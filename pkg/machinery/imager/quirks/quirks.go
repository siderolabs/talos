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

	// we only care about major, minor, and patch, so that alpha, beta, etc. are ignored
	return Quirks{v: &semver.Version{
		Major: v.Major,
		Minor: v.Minor,
		Patch: v.Patch,
	}}
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

var minVersionOverlay = semver.MustParse("1.7.0")

// SupportsOverlay returns true if the Talos imager version supports overlay.
func (q Quirks) SupportsOverlay() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return true
	}

	return q.v.GTE(minVersionOverlay)
}

var minVersionZstd = semver.MustParse("1.8.0")

// UseZSTDCompression returns true if the Talos should use zstd compression in place of xz.
func (q Quirks) UseZSTDCompression() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return true
	}

	return q.v.GTE(minVersionZstd)
}

var minVersionISOLabel = semver.MustParse("1.8.0")

// SupportsISOLabel returns true if the Talos version supports setting the ISO label.
func (q Quirks) SupportsISOLabel() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return true
	}

	return q.v.GTE(minVersionISOLabel)
}

var minVersionMultidoc = semver.MustParse("1.5.0")

// SupportsMultidoc returns true if the Talos version supports multidoc machine configs.
func (q Quirks) SupportsMultidoc() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return true
	}

	return q.v.GTE(minVersionMultidoc)
}

// maxVersionMetalPlatformConsoleTTYS0Dropped is the version that dropped console=ttyS0 for metal image.
var maxVersionMetalPlatformConsoleTTYS0Dropped = semver.MustParse("1.8.0")

// SupportsMetalPlatformConsoleTTYS0 returns true if the Talos version supports already has console=ttyS0 kernel argument.
func (q Quirks) SupportsMetalPlatformConsoleTTYS0() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return false
	}

	return q.v.LT(maxVersionMetalPlatformConsoleTTYS0Dropped)
}

// minVersionSupportsHalfIfInstalled is the version that supports half if installed.
var minVersionSupportsHalfIfInstalled = semver.MustParse("1.8.0")

// SupportsHaltIfInstalled returns true if the Talos version supports half if installed.
func (q Quirks) SupportsHaltIfInstalled() bool {
	return q.v.GTE(minVersionSupportsHalfIfInstalled)
}

var minVersionSkipDataPartitions = semver.MustParse("1.8.0")

// SkipDataPartitions returns true if the Talos version supports creating EPHEMERAL/STATE partitions on its own.
func (q Quirks) SkipDataPartitions() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return true
	}

	return q.v.GTE(minVersionSkipDataPartitions)
}

// minVersionSELinux is the version that enabled SELinux and added respective parameters.
var minVersionSELinux = semver.MustParse("1.9.0")

// SupportsSELinux returns true if the Talos version supports already has console=ttyS0 kernel argument.
func (q Quirks) SupportsSELinux() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return true
	}

	return q.v.GTE(minVersionSELinux)
}
