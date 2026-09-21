// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import "github.com/siderolabs/talos/pkg/machinery/config/types/meta"

// Size types are shared with other config documents, so they live in the meta package.
//
// These aliases are kept for backwards compatibility.
type (
	// ByteSize is a byte size which can be conveniently represented as a human readable string
	// with IEC sizes, e.g. 100MB.
	ByteSize = meta.ByteSize
	// PercentageSize is a size in percents.
	PercentageSize = meta.PercentageSize
	// Size is either a PercentageSize or ByteSize.
	Size = meta.Size
)

// MustSize returns a new Size with the given value.
//
// It panics if the value is invalid.
func MustSize(value string) Size {
	return meta.MustSize(value)
}

// MustByteSize returns a new Size with the given ByteSize value.
//
// It panics if the value is invalid.
func MustByteSize(value string) ByteSize {
	return meta.MustByteSize(value)
}
