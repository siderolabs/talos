// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pe

import (
	"debug/pe"
	"fmt"
	"io"
)

// AssetInfo contains the kernel, initrd, and cmdline from a PE file.
type AssetInfo struct {
	io.Closer

	Kernel  io.Reader
	Initrd  io.Reader
	Cmdline io.Reader
}

// Extract extracts the kernel, initrd, and cmdline from a PE file.
func Extract(ukiPath string) (assetInfo AssetInfo, err error) {
	peFile, err := pe.Open(ukiPath)
	if err != nil {
		return assetInfo, fmt.Errorf("failed to open PE file: %w", err)
	}

	assetInfo.Closer = peFile

	sectionMap := map[string]*io.Reader{
		".initrd":  &assetInfo.Initrd,
		".cmdline": &assetInfo.Cmdline,
		".linux":   &assetInfo.Kernel,
	}

	for _, section := range peFile.Sections {
		if reader, exists := sectionMap[section.Name]; exists && *reader == nil {
			*reader = io.LimitReader(section.Open(), int64(section.VirtualSize))
		}
	}

	for name, reader := range sectionMap {
		if *reader == nil {
			return assetInfo, fmt.Errorf("%s not found in PE file", name)
		}
	}

	return assetInfo, nil
}
