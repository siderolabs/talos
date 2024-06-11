// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package celenv provides standard CEL environments to evaluate CEL expressions.
package celenv

import (
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"

	"github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
)

// DiskLocator is a disk locator CEL environment.
var DiskLocator = sync.OnceValue(func() *cel.Env {
	var diskSpec block.DiskSpec

	env, err := cel.NewEnv(
		cel.Types(&diskSpec),
		cel.Variable("disk", cel.ObjectType(string(diskSpec.ProtoReflect().Descriptor().FullName()))),
		cel.Variable("system_disk", types.BoolType),
	)
	if err != nil {
		panic(err)
	}

	return env
})

// VolumeLocator is a volume locator CEL environment.
var VolumeLocator = sync.OnceValue(func() *cel.Env {
	var volumeSpec block.DiscoveredVolumeSpec

	env, err := cel.NewEnv(
		cel.Types(&volumeSpec),
		cel.Variable("volume", cel.ObjectType(string(volumeSpec.ProtoReflect().Descriptor().FullName()))),
	)
	if err != nil {
		panic(err)
	}

	return env
})
