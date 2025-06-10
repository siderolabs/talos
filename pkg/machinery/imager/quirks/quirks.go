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

// Version returns the Talos version.
func (q Quirks) Version() *semver.Version {
	return q.v
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

var minVersionUKI = semver.MustParse("1.5.0")

// SupportsUKI returns true if the Talos version supports building UKIs.
func (q Quirks) SupportsUKI() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return true
	}

	return q.v.GTE(minVersionUKI)
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
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return true
	}

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
var minVersionSELinux = semver.MustParse("1.10.0")

// SupportsSELinux returns true if the Talos version enables selinux=1 by default.
func (q Quirks) SupportsSELinux() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return true
	}

	return q.v.GTE(minVersionSELinux)
}

// minVersionUseSDBootOnly is the version that supports only SDBoot for UEFI.
var minTalosVersionUseSDBootOnly = semver.MustParse("1.10.0")

// UseSDBootForUEFI returns true if the Talos version supports only SDBoot for UEFI.
func (q Quirks) UseSDBootForUEFI() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return false
	}

	return q.v.GTE(minTalosVersionUseSDBootOnly)
}

// minTalosVersionUsrMerge is the version that has /lib and /bin symlinked into /usr.
var minTalosVersionUsrMerge = semver.MustParse("1.10.0")

// KernelModulesPath returns kernel module storage path for the given Talos version.
func (q Quirks) KernelModulesPath() string {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil || q.v.GTE(minTalosVersionUsrMerge) {
		return "/usr/lib/modules"
	}

	return "/lib/modules"
}

// FirmwarePath returns firmware storage path for the given Talos version.
func (q Quirks) FirmwarePath() string {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil || q.v.GTE(minTalosVersionUsrMerge) {
		return "/usr/lib/firmware"
	}

	return "/lib/firmware"
}

// minTalosVersionUKIProfiles is the version that supports UKI profiles.
var minTalosVersionUKIProfiles = semver.MustParse("1.10.0")

// SupportsUKIProfiles returns true if the Talos version supports UKI profiles.
func (q Quirks) SupportsUKIProfiles() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return true
	}

	return q.v.GTE(minTalosVersionUKIProfiles)
}

var minTalosVersionUnifiedInstaller = semver.MustParse("1.10.0")

// SupportsUnifiedInstaller returns true if the Talos version supports unified installer.
func (q Quirks) SupportsUnifiedInstaller() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return true
	}

	return q.v.GTE(minTalosVersionUnifiedInstaller)
}

// XFSMkfsConfig returns the mkfs.xfs config for the given Talos version.
func (q Quirks) XFSMkfsConfig() string {
	switch version := q.v; {
	// if the version doesn't parse, we assume it's latest Talos
	// update when we have a new LTS config
	case version == nil:
		return "/usr/share/xfsprogs/mkfs/lts_6.12.conf"
	// add new version once we have a new LTS config
	case version.GTE(semver.MustParse("1.10.0")):
		return "/usr/share/xfsprogs/mkfs/lts_6.12.conf"
	case version.GTE(semver.MustParse("1.8.0")) && version.LT(semver.MustParse("1.10.0")):
		return "/usr/share/xfsprogs/mkfs/lts_6.6.conf"
	case version.GTE(semver.MustParse("1.5.0")) && version.LT(semver.MustParse("1.8.0")):
		return "/usr/share/xfsprogs/mkfs/lts_6.1.conf"
	default:
		return "/usr/share/xfsprogs/mkfs/lts_6.1.conf"
	}
}

var maxTalosVersionIMASupported = semver.MustParse("1.10.99")

// SupportsIMA returns true if the Talos version has IMA support.
func (q Quirks) SupportsIMA() bool {
	// if the version doesn't parse, we assume it's latest Talos
	if q.v == nil {
		return false
	}

	return q.v.LTE(maxTalosVersionIMASupported)
}
