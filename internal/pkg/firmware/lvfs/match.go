// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvfs

import (
	"cmp"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// VersionFormatPlain is the LVFS version format whose versions compare as
// strings; NVMe firmware revisions use it.
const VersionFormatPlain = "plain"

// Device describes an installed device for matching against the catalog.
type Device struct {
	// GUIDs are the fwupd-compatible GUIDs of the device's instance IDs.
	GUIDs []string
	// Protocol is the update protocol the device supports, e.g. org.nvmexpress.
	Protocol string
	// Version is the installed firmware version.
	Version string
	// VendorIDs are the fwupd-style vendor IDs of the device, e.g. NVME:0x15B7.
	VendorIDs []string
	// HardwareIDs are the computer hardware IDs (CHIDs) of the machine the
	// device is in. Components restricted to specific hardware are only
	// offered when one of them matches, so never when this is empty.
	HardwareIDs []string
}

// Update is a release of a catalog component newer than what a device runs.
type Update struct {
	Component *Component
	Release   Release
}

// Updates returns the highest-versioned release of every component that
// applies to the device, when it is newer than the installed version.
//
// A component applies when it provides one of the device's GUIDs, uses the
// device's protocol, and all of its requirements hold: firmware requirements
// on the installed version and on the vendor ID, and hardware requirements,
// are evaluated; the requirement on the fwupd version is ignored; any other
// requirement disqualifies the component.
//
// A device whose installed version is unknown gets no updates, as nothing can
// be said to be newer than it.
func (c *Catalog) Updates(dev Device) []Update {
	if dev.Version == "" {
		return nil
	}

	var updates []Update

	for _, component := range c.applicable(dev) {
		format := component.VersionFormat()

		release, ok := component.highestRelease(format)
		if !ok || CompareVersions(format, release.Version, dev.Version) <= 0 {
			continue
		}

		updates = append(updates, Update{Component: component, Release: release})
	}

	return updates
}

// Release returns the release of the given version from every component that
// applies to the device, as Updates defines it.
//
// The version is not compared to the installed one: the release may be older
// than, or the same as, what the device runs, and whether to allow that is up
// to the caller.
func (c *Catalog) Release(dev Device, version string) []Update {
	var found []Update

	for _, component := range c.applicable(dev) {
		for _, release := range component.Releases {
			if release.Version == version {
				found = append(found, Update{Component: component, Release: release})

				break
			}
		}
	}

	return found
}

// applicable returns the components that apply to the device.
func (c *Catalog) applicable(dev Device) []*Component {
	var components []*Component

	for _, component := range c.Lookup(dev.GUIDs) {
		if component.Protocol() == dev.Protocol && component.applies(dev) {
			components = append(components, component)
		}
	}

	return components
}

// highestRelease returns the release with the highest version; publication
// order doesn't matter, as maintenance releases of old branches come late.
func (c *Component) highestRelease(format string) (Release, bool) {
	if len(c.Releases) == 0 {
		return Release{}, false
	}

	return slices.MaxFunc(c.Releases, func(a, b Release) int {
		return CompareVersions(format, a.Version, b.Version)
	}), true
}

// VersionFormat returns the LVFS version format of the component's versions,
// VersionFormatPlain when unspecified.
func (c *Component) VersionFormat() string {
	for _, v := range c.Custom {
		if v.Key == "LVFS::VersionFormat" {
			if format := strings.TrimSpace(v.Value); format != "" {
				return format
			}
		}
	}

	return VersionFormatPlain
}

// fwupdID is the requirement ID of the fwupd daemon version.
const fwupdID = "org.freedesktop.fwupd"

// applies reports whether the component's requirements hold for the device.
func (c *Component) applies(dev Device) bool {
	format := c.VersionFormat()

	for _, req := range c.Requires.Firmwares {
		if !firmwareHolds(&req, format, dev) {
			return false
		}
	}

	for _, req := range c.Requires.Others {
		if !requirementHolds(&req, dev) {
			return false
		}
	}

	return true
}

// requirementHolds evaluates a non-firmware requirement.
func requirementHolds(req *Requirement, dev Device) bool {
	value := strings.TrimSpace(req.Value)

	switch req.XMLName.Local {
	case "hardware":
		return hasHardwareID(dev, value)
	case "not_hardware":
		// without hardware IDs the exclusion can't be ruled out
		return len(dev.HardwareIDs) != 0 && !hasHardwareID(dev, value)
	case "id":
		// the firmware is written natively, not by fwupd; requirements on
		// other software can't be evaluated
		return value == fwupdID
	default:
		return false
	}
}

// firmwareHolds evaluates a firmware requirement.
func firmwareHolds(req *FirmwareRequirement, format string, dev Device) bool {
	// depth 0 is the device itself; a requirement on a parent device can't
	// be evaluated
	if depth := strings.TrimSpace(req.Depth); depth != "" && depth != "0" {
		return false
	}

	switch strings.TrimSpace(req.Value) {
	case "":
		// nothing can be said about an unknown installed version
		return dev.Version != "" && matches(req.Compare, format, dev.Version, req.Version)
	case "vendor-id":
		return slices.ContainsFunc(dev.VendorIDs, func(id string) bool {
			return matches(req.Compare, VersionFormatPlain, id, req.Version)
		})
	default:
		// a requirement on other firmware, e.g. a bootloader, can't be evaluated
		return false
	}
}

// hasHardwareID reports whether the device's machine has one of the
// |-separated hardware IDs.
func hasHardwareID(dev Device, ids string) bool {
	for id := range strings.SplitSeq(ids, "|") {
		if slices.ContainsFunc(dev.HardwareIDs, func(have string) bool {
			return strings.EqualFold(have, strings.TrimSpace(id))
		}) {
			return true
		}
	}

	return false
}

// comparisons maps the ordering operators of LVFS requirements to a check of
// the CompareVersions result.
var comparisons = map[string]func(int) bool{
	"eq": func(c int) bool { return c == 0 },
	"ne": func(c int) bool { return c != 0 },
	"lt": func(c int) bool { return c < 0 },
	"le": func(c int) bool { return c <= 0 },
	"gt": func(c int) bool { return c > 0 },
	"ge": func(c int) bool { return c >= 0 },
}

// matches evaluates an LVFS requirement comparison of an actual value against
// the required one.
func matches(compare, format, actual, required string) bool {
	switch compare {
	case "glob":
		ok, err := path.Match(required, actual)

		return err == nil && ok
	case "regex":
		re, err := regexp.Compile(required)

		return err == nil && re.MatchString(actual)
	}

	check, ok := comparisons[compare]
	if !ok {
		return false
	}

	return check(CompareVersions(format, actual, required))
}

// CompareVersions orders two versions of an LVFS version format.
//
// Plain versions compare as strings. Versions of every other format compare
// as dot-separated segments, numerically where both segments are numbers
// (decimal, or hexadecimal with a 0x prefix) and as strings otherwise; a
// missing segment sorts before any present one.
func CompareVersions(format, a, b string) int {
	if format == VersionFormatPlain {
		return strings.Compare(a, b)
	}

	as, bs := strings.Split(a, "."), strings.Split(b, ".")

	for i := range max(len(as), len(bs)) {
		var x, y string

		if i < len(as) {
			x = as[i]
		}

		if i < len(bs) {
			y = bs[i]
		}

		if c := compareSegment(x, y); c != 0 {
			return c
		}
	}

	return 0
}

func compareSegment(x, y string) int {
	xn, xok := segmentNumber(x)
	yn, yok := segmentNumber(y)

	if xok && yok {
		return cmp.Compare(xn, yn)
	}

	return strings.Compare(x, y)
}

// segmentNumber parses a version segment as a decimal number, or as a
// hexadecimal one when it carries the 0x prefix of the hex version format.
func segmentNumber(s string) (uint64, bool) {
	base := 10

	if hex, ok := strings.CutPrefix(strings.ToLower(s), "0x"); ok {
		s, base = hex, 16
	}

	n, err := strconv.ParseUint(s, base, 64)

	return n, err == nil
}
