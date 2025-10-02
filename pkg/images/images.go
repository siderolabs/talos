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

	// DefaultInstallerImageName is the default container image name for
	// the installer.
	DefaultInstallerImageName = Username + "/installer"

	// DefaultInstallerImageRepository is the default container repository for
	// the installer.
	DefaultInstallerImageRepository = Registry + "/" + DefaultInstallerImageName

	// DefaultInstallerImage is the default installer image.
	DefaultInstallerImage = DefaultInstallerImageRepository + ":" + version.Tag

	// DefaultTalosImageRepository is the default container repository for
	// the talos image.
	DefaultTalosImageRepository = Registry + "/" + Username + "/talos"

	// DefaultTalosImage is the default talos image.
	DefaultTalosImage = DefaultTalosImageRepository + ":" + version.Tag

	// DefaultInstallerBaseImageRepository is the default container repository for
	// installer-base image.
	DefaultInstallerBaseImageRepository = Registry + "/" + Username + "/installer-base"

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
