// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile

import (
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/meta"
)

// CustomizationProfile describes customizations that can be applied to the image.
type CustomizationProfile struct {
	// ExtraKernelArgs is a list of extra kernel arguments.
	ExtraKernelArgs []string `yaml:"extraKernelArgs,omitempty"`
	// MetaContents is a list of META partition contents.
	MetaContents meta.Values `yaml:"metaContents,omitempty"`
}

// FillDefaults fills the default values for the customization profile.
func (c *CustomizationProfile) FillDefaults(outKind OutputKind, arch, version string, secureboot bool) {
	// if secureboot is enabled or the architecture is arm64, we don't need to do anything as only sd-boot is installed
	if secureboot || arch == "arm64" {
		return
	}

	if outKind == OutKindImage && quirks.New(version).UseSDBootForUEFI() {
		if c.MetaContents == nil {
			c.MetaContents = meta.Values{}
		}

		c.MetaContents = append(c.MetaContents, meta.Value{
			Key:   meta.DiskImageBootloader,
			Value: DiskImageBootloaderDualBoot.String(),
		})
	}
}
