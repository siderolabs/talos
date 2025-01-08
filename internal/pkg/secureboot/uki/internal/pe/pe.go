// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package pe handles appending sections to PE files.
package pe

import (
	"github.com/siderolabs/talos/internal/pkg/secureboot"
)

// Section is a UKI file section.
type Section struct {
	// Section name.
	Name secureboot.Section
	// Path to the contents of the section.
	Path string
	// Should the section be measured to the TPM?
	Measure bool
	// Should the section be appended, or is it already in the PE file.
	Append bool
	// Virtual virtualSize & VMA of the section.
	virtualSize    uint64
	virtualAddress uint64
}
