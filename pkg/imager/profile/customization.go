// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile

import (
	"github.com/siderolabs/talos/pkg/machinery/meta"
)

// CustomizationProfile describes customizations that can be applied to the image.
type CustomizationProfile struct {
	// ExtraKernelArgs is a list of extra kernel arguments.
	ExtraKernelArgs []string `yaml:"extraKernelArgs,omitempty"`
	// MetaContents is a list of META partition contents.
	MetaContents meta.Values `yaml:"metaContents,omitempty"`
	// EmbeddedMachineConfiguration is the machine configuration to embed into the image.
	EmbeddedMachineConfiguration string `yaml:"embeddedMachineConfiguration,omitempty"`
}
