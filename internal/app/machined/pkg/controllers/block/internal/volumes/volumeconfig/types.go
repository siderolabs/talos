// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig

import (
	"fmt"

	machinedruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// VolumeResource is an internal type containing the information required by
// VolumeConfigController to create a VolumeConfig (and optionally
// VolumeMountRequest) resource.
//
// Transformers (System and User) transform the config into VolumeResources, which are
// later created by VolumeConfigController.
type VolumeResource struct {
	VolumeID           string                                  // ID of the volume to create.
	Label              string                                  // label of the volume to create.
	TransformFunc      func(vc *block.VolumeConfig) error      // func that applies the changes to the provided VolumeConfig.
	MountTransformFunc func(m *block.VolumeMountRequest) error // func that applies the changes to the provided VolumeMountRequest.
}

type volumeConfigTransformer func(c configconfig.Config) ([]VolumeResource, error)

// SkipUserVolumeMountRequest is used to skip creating a VolumeMountRequest for a user volume.
type SkipUserVolumeMountRequest struct{}

var noMatch = cel.MustExpression(cel.ParseBooleanExpression("false", celenv.Empty()))

func labelVolumeMatch(label string) cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(fmt.Sprintf("volume.partition_label == '%s'", label), celenv.VolumeLocator()))
}

func labelVolumeMatchAndNonEmpty(label string) cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(fmt.Sprintf("volume.partition_label == '%s' && volume.name != ''", label), celenv.VolumeLocator()))
}

func metaMatch() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(fmt.Sprintf("volume.partition_label == '%s' && volume.name in ['', 'talosmeta'] && volume.size == 1048576u", constants.MetaPartitionLabel), celenv.VolumeLocator())) //nolint:lll
}

func systemDiskMatch() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression("system_disk", celenv.DiskLocator()))
}

// MetaProvider wraps acquiring meta.
type MetaProvider interface {
	Meta() machinedruntime.Meta
}
