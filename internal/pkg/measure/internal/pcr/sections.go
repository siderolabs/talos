// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pcr

// OrderedSections returns the sections that are measured into PCR.
//
// Derived from https://github.com/systemd/systemd/blob/v257.1/src/fundamental/uki.h#L6
// .pcrsig section is omitted here since that's what we are calulating here.
func OrderedSections() []string {
	// DO NOT REARRANGE
	return []string{
		".linux",
		".osrel",
		".cmdline",
		".initrd",
		".ucode",
		".splash",
		".dtb",
		".uname",
		".sbat",
		".pcrpkey",
		".profile",
		".dtbauto",
		".hwids",
	}
}
