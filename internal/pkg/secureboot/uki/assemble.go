// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package uki

import (
	"path/filepath"

	"github.com/siderolabs/talos/internal/pkg/secureboot/uki/internal/pe"
)

// assemble the UKI file out of sections.
func (builder *Builder) assemble() error {
	builder.unsignedUKIPath = filepath.Join(builder.scratchDir, "unsigned.uki")

	return pe.AssembleNative(builder.SdStubPath, builder.unsignedUKIPath, builder.sections)
}
