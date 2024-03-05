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
	DefaultTalosImageRepository = Registry + "/" + Username + "/" + "talos"
)
