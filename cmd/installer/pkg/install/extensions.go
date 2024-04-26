// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package install provides the installation routine.
package install

import (
	"fmt"
	"log"

	"github.com/siderolabs/talos/pkg/imager/extensions"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

func (i *Installer) installExtensions() error {
	builder := extensions.Builder{
		InitramfsPath:     fmt.Sprintf(constants.InitramfsAssetPath, i.options.Arch),
		Arch:              i.options.Arch,
		ExtensionTreePath: constants.SystemExtensionsPath,
		Printf:            log.Printf,
		Quirks:            quirks.New(i.options.Version),
	}

	return builder.Build()
}
