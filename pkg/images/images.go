// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package images provides some default images.
package images

import (
	"github.com/siderolabs/talos/pkg/machinery/gendata"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

var (
	// Username is the default registry username.
	Username = gendata.ImagesUsername

	// Registry is the default registry.
	Registry = gendata.ImagesRegistry

	// Factory is the default factory for images.
	Factory = gendata.ImageFactory

	// DefaultInstallerImageSchematic is the default (empty) image schematic for the installer.
	DefaultInstallerImageSchematic = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

	// DefaultInstallerImageName is the default container image name for
	// the installer.
	//
	// Deprecated: This image is only used for legacy installer for Talos <1.11.0 and during tests.
	DefaultInstallerImageName = Username + "/installer"

	// DefaultInstallerImageRepository is the default container repository for
	// the installer.
	//
	// Deprecated: This image is only used for legacy installer for Talos <1.11.0 and during tests.
	DefaultInstallerImageRepository = Registry + "/" + DefaultInstallerImageName

	// DefaultInstallerBaseImageRepository is the default container repository for
	// installer-base image.
	DefaultInstallerBaseImageRepository = Registry + "/" + Username + "/installer-base"

	// DefaultTalosImageName is the default container image name for
	// the talos image.
	DefaultTalosImageName = Username + "/talos"

	// DefaultTalosImageRepository is the default container repository for
	// the talos image.
	DefaultTalosImageRepository = Registry + "/" + DefaultTalosImageName

	// DefaultTalosImage is the default talos image.
	DefaultTalosImage = DefaultTalosImageRepository + ":" + version.Tag

	// DefaultImagerImageRepository is the default container repository for
	// imager image.
	DefaultImagerImageRepository = Registry + "/" + Username + "/imager"

	// DefaultTalosctlAllImageRepository is the default container repository for
	// talosctl-all image.
	DefaultTalosctlAllImageRepository = Registry + "/" + Username + "/talosctl-all"

	// DefaultOverlaysManifestName is the default container manifest name for
	// the overlays.
	DefaultOverlaysManifestName = Username + "/overlays"

	// DefaultOverlaysManifestRepository is the default container repository for
	// overlays manifest.
	DefaultOverlaysManifestRepository = Registry + "/" + DefaultOverlaysManifestName

	// DefaultExtensionsManifestName is the default container manifest name for
	// the extensions.
	DefaultExtensionsManifestName = Username + "/extensions"

	// DefaultExtensionsManifestRepository is the default container repository for
	// extensions manifest.
	DefaultExtensionsManifestRepository = Registry + "/" + DefaultExtensionsManifestName
)

// SecureBoot selects whether the secureboot variant of the installer image is used.
type SecureBoot bool

// SecureBoot options for the installer image.
const (
	NoSecureBoot   SecureBoot = false
	WithSecureBoot SecureBoot = true
)

// InstallerImageRepository returns the default container repository for the installer for the given platform.
func InstallerImageRepository(platform string) string {
	return NewInstallerImageRepository(Factory, platform, DefaultInstallerImageSchematic, NoSecureBoot)
}

// InstallerImage returns the default installer image for the given platform.
func InstallerImage(platform string) string {
	return NewInstallerImage(Factory, platform, DefaultInstallerImageSchematic, version.Tag, NoSecureBoot)
}

// NewInstallerImageRepository builds a new installer image repository of the form
// <factory>/<platform>-installer[-secureboot]/<schematic>.
//
// If factory is empty, the default Image Factory is used.
func NewInstallerImageRepository(factory, platform, schematic string, secureBoot SecureBoot) string {
	if factory == "" {
		factory = Factory
	}

	installerType := platform + "-installer"
	if secureBoot {
		installerType += "-secureboot"
	}

	return factory + "/" + installerType + "/" + schematic
}

// NewInstallerImage builds a new installer image of the form
// <factory>/<platform>-installer[-secureboot]/<schematic>:<tag>.
//
// If factory is empty, the default Image Factory is used; if tag is empty, the current Talos version is used.
func NewInstallerImage(factory, platform, schematic, tag string, secureBoot SecureBoot) string {
	if factory == "" {
		factory = Factory
	}

	if tag == "" {
		tag = version.Tag
	}

	return NewInstallerImageRepository(factory, platform, schematic, secureBoot) + ":" + tag
}
