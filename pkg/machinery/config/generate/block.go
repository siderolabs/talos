// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func (in *Input) generateBlockConfigs() []config.Document {
	if !in.Options.VersionContract.FilesystemTrimEnabledByDefault() {
		return nil
	}

	ephemeralConfig := block.NewVolumeConfigV1Alpha1()
	ephemeralConfig.MetaName = constants.EphemeralPartitionLabel
	ephemeralConfig.MountSpec.MountSecure = new(true)

	trimConfig := block.NewFilesystemTrimConfigV1Alpha1()
	trimConfig.TrimInterval = constants.DefaultFilesystemTrimInterval

	return []config.Document{ephemeralConfig, trimConfig}
}
