// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package celenv provides standard CEL environments to evaluate CEL expressions.
package celenv

import (
	"net"
	"slices"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/ryanuber/go-glob"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Empty is an empty CEL environment.
var Empty = sync.OnceValue(func() *cel.Env {
	env, err := cel.NewEnv()
	if err != nil {
		panic(err)
	}

	return env
})

// DiskLocator is a disk locator CEL environment.
var DiskLocator = sync.OnceValue(func() *cel.Env {
	var diskSpec block.DiskSpec

	env, err := cel.NewEnv(
		slices.Concat(
			[]cel.EnvOption{
				cel.Types(&diskSpec),
				cel.Variable("disk", cel.ObjectType(string(diskSpec.ProtoReflect().Descriptor().FullName()))),
				cel.Variable("system_disk", types.BoolType),
				cel.Function("glob", // glob(pattern, string)
					cel.Overload("glob_string_string", []*cel.Type{cel.StringType, cel.StringType}, cel.BoolType,
						cel.BinaryBinding(func(arg1, arg2 ref.Val) ref.Val {
							return types.Bool(glob.Glob(string(arg1.(types.String)), string(arg2.(types.String))))
						}),
					),
				),
			},
			celUnitMultipliersConstants(),
		)...,
	)
	if err != nil {
		panic(err)
	}

	return env
})

// VolumeLocator is a volume locator CEL environment.
var VolumeLocator = sync.OnceValue(func() *cel.Env {
	var (
		volumeSpec block.DiscoveredVolumeSpec
		diskSpec   block.DiskSpec
	)

	env, err := cel.NewEnv(
		slices.Concat(
			[]cel.EnvOption{
				cel.Types(&volumeSpec),
				cel.Types(&diskSpec),
				cel.Variable("volume", cel.ObjectType(string(volumeSpec.ProtoReflect().Descriptor().FullName()))),
				cel.Variable("disk", cel.ObjectType(string(diskSpec.ProtoReflect().Descriptor().FullName()))),
				cel.Variable("system_disk", types.BoolType),
			},
			celUnitMultipliersConstants(),
		)...,
	)
	if err != nil {
		panic(err)
	}

	return env
})

// OOMTrigger is a OOM Trigger Condition CEL environment.
var OOMTrigger = sync.OnceValue(func() *cel.Env {
	env, err := cel.NewEnv(
		slices.Concat(
			slices.Concat(
				[]cel.EnvOption{
					cel.Variable("memory_some_avg10", types.DoubleType),
					cel.Variable("memory_some_avg60", types.DoubleType),
					cel.Variable("memory_some_avg300", types.DoubleType),
					cel.Variable("memory_some_total", types.DoubleType),
					cel.Variable("memory_full_avg10", types.DoubleType),
					cel.Variable("memory_full_avg60", types.DoubleType),
					cel.Variable("memory_full_avg300", types.DoubleType),
					cel.Variable("memory_full_total", types.DoubleType),
					cel.Variable("d_memory_some_avg10", types.DoubleType),
					cel.Variable("d_memory_some_avg60", types.DoubleType),
					cel.Variable("d_memory_some_avg300", types.DoubleType),
					cel.Variable("d_memory_some_total", types.DoubleType),
					cel.Variable("d_memory_full_avg10", types.DoubleType),
					cel.Variable("d_memory_full_avg60", types.DoubleType),
					cel.Variable("d_memory_full_avg300", types.DoubleType),
					cel.Variable("d_memory_full_total", types.DoubleType),
					cel.Variable("time_since_trigger", types.DurationType),
					cel.OptionalTypes(),
				},
				celUnitMultipliersConstants(),
			),
			celCgroupClassConstants(),
		)...,
	)
	if err != nil {
		panic(err)
	}

	return env
})

// OOMCgroupScoring is a OOM Cgroup Scoring CEL environment.
var OOMCgroupScoring = sync.OnceValue(func() *cel.Env {
	env, err := cel.NewEnv(
		slices.Concat(
			slices.Concat(
				[]cel.EnvOption{
					cel.Variable("memory_max", types.NewOptionalType(types.UintType)),
					cel.Variable("memory_current", types.NewOptionalType(types.UintType)),
					cel.Variable("memory_peak", types.NewOptionalType(types.UintType)),
					cel.Variable("path", types.StringType),
					cel.Variable("class", types.IntType),
					cel.OptionalTypes(),
				},
				celUnitMultipliersConstants(),
			),
			celCgroupClassConstants(),
		)...,
	)
	if err != nil {
		panic(err)
	}

	return env
})

// LinkLocator is a network link locator CEL environment.
var LinkLocator = sync.OnceValue(func() *cel.Env {
	var linkSpec network.LinkStatusSpec

	env, err := cel.NewEnv(
		slices.Concat(
			[]cel.EnvOption{
				cel.Types(&linkSpec),
				cel.Variable("link", cel.ObjectType(string(linkSpec.ProtoReflect().Descriptor().FullName()))),
				cel.Function("glob", // glob(pattern, string) -> bool
					cel.Overload("glob_string_string", []*cel.Type{cel.StringType, cel.StringType}, cel.BoolType,
						cel.BinaryBinding(func(arg1, arg2 ref.Val) ref.Val {
							return types.Bool(glob.Glob(string(arg1.(types.String)), string(arg2.(types.String))))
						}),
					),
				),
				cel.Function("mac", // mac(bytes) -> string
					cel.Overload("mac_bytes", []*cel.Type{cel.BytesType}, cel.StringType,
						cel.UnaryBinding(func(arg ref.Val) ref.Val {
							return types.String(net.HardwareAddr([]byte(arg.(types.Bytes))).String())
						}),
					),
				),
			},
		)...,
	)
	if err != nil {
		panic(err)
	}

	return env
})

type unitMultiplier struct {
	unit       string
	multiplier uint64
}

var unitMultipliers = []unitMultiplier{
	// IEC.
	{"KiB", 1024},
	{"MiB", 1024 * 1024},
	{"GiB", 1024 * 1024 * 1024},
	{"TiB", 1024 * 1024 * 1024 * 1024},
	{"PiB", 1024 * 1024 * 1024 * 1024 * 1024},
	{"EiB", 1024 * 1024 * 1024 * 1024 * 1024 * 1024},
	// Metric (used for disk sizes).
	{"kB", 1000},
	{"MB", 1000 * 1000},
	{"GB", 1000 * 1000 * 1000},
	{"TB", 1000 * 1000 * 1000 * 1000},
	{"PB", 1000 * 1000 * 1000 * 1000 * 1000},
	{"EB", 1000 * 1000 * 1000 * 1000 * 1000 * 1000},
}

func celUnitMultipliersConstants() []cel.EnvOption {
	return xslices.Map(unitMultipliers, func(um unitMultiplier) cel.EnvOption {
		return cel.Constant(um.unit, types.UintType, types.Uint(um.multiplier))
	})
}

func celCgroupClassConstants() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Constant("Besteffort", types.IntType, types.Int(runtime.QoSCgroupClassBesteffort)),
		cel.Constant("Burstable", types.IntType, types.Int(runtime.QoSCgroupClassBurstable)),
		cel.Constant("Guaranteed", types.IntType, types.Int(runtime.QoSCgroupClassGuaranteed)),
		cel.Constant("Podruntime", types.IntType, types.Int(runtime.QoSCgroupClassPodruntime)),
		cel.Constant("System", types.IntType, types.Int(runtime.QoSCgroupClassSystem)),
	}
}
