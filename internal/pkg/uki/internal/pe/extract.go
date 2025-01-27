// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pe

import (
	"debug/pe"
	"fmt"
	"io"
)

// fileCloser is an interface that wraps the Close method.
type fileCloser interface {
	Close() error
}

// AssetInfo contains the kernel, initrd, and cmdline from a PE file.
type AssetInfo struct {
	Kernel  io.ReadSeeker
	Initrd  io.ReadSeeker
	Cmdline io.ReadSeeker
	fileCloser
}

// Extract extracts the kernel, initrd, and cmdline from a PE file.
func Extract(ukiPath string) (assetInfo AssetInfo, err error) {
	peFile, err := pe.Open(ukiPath)
	if err != nil {
		return assetInfo, fmt.Errorf("failed to open PE file: %w", err)
	}

	assetInfo.fileCloser = peFile

	for _, section := range peFile.Sections {
		// read upto section.VirtualSize bytes
		sectionReader := io.NewSectionReader(section, 0, int64(section.VirtualSize))

		switch section.Name {
		case ".initrd":
			assetInfo.Initrd = sectionReader
		case ".cmdline":
			assetInfo.Cmdline = sectionReader
		case ".linux":
			assetInfo.Kernel = sectionReader
		}
	}

	if assetInfo.Kernel == nil {
		return assetInfo, fmt.Errorf("kernel not found in PE file")
	}

	if assetInfo.Initrd == nil {
		return assetInfo, fmt.Errorf("initrd not found in PE file")
	}

	if assetInfo.Cmdline == nil {
		return assetInfo, fmt.Errorf("cmdline not found in PE file")
	}

	return assetInfo, nil
}
