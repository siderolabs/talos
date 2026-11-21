// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile_test

import (
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestAMD64Profiles(t *testing.T) {
	builtins := profile.Profiles(constants.ArchAmd64)
	require.Len(t, builtins, 3)

	for index, expected := range []struct {
		name     string
		machine  string
		mode     hypervisorhelpers.VirtualMachineProfileCPUMode
		model    string
		fallback string
	}{
		{
			name:    "host",
			machine: "q35",
			mode:    hypervisorhelpers.VirtualMachineProfileCPUModeHostModel,
		},
		{
			name:     "legacy",
			machine:  "pc-i440fx-9.2",
			mode:     hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
			model:    "kvm64",
			fallback: "forbid",
		},
		{
			name:    "passthrough",
			machine: "q35",
			mode:    hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough,
		},
	} {
		builtin := builtins[index]
		require.Equal(t, expected.name, builtin.Metadata().ID())
		require.Equal(t, hypervisor.NamespaceName, builtin.Metadata().Namespace())
		require.Equal(t, hypervisor.VirtualMachineProfileSpecSpec{
			Builtin: true,
			Machine: expected.machine,
			CPU: hypervisor.VirtualMachineProfileCPUSpec{
				Mode:     expected.mode,
				Model:    expected.model,
				Fallback: expected.fallback,
			},
			Timers: hypervisor.DefaultVirtualMachineProfileTimersSpec(),
		}, *builtin.TypedSpec())
		require.NoError(t, profile.Validate(constants.ArchAmd64, expected.name, *builtin.TypedSpec()))
	}

	builtins[0].TypedSpec().CPU.Features = []hypervisor.VirtualMachineProfileCPUFeatureSpec{
		{
			Name: "changed",
		},
	}
	fresh := profile.Profiles(constants.ArchAmd64)
	require.Equal(t, "host", fresh[0].Metadata().ID())
	require.Equal(t, "q35", fresh[0].TypedSpec().Machine)
	require.Empty(t, fresh[0].TypedSpec().CPU.Features)
}

// builtinContractMutations changes one hardware field each, keyed by its path in the spec.
var builtinContractMutations = map[string]func(*hypervisor.VirtualMachineProfileSpecSpec){
	"Machine": func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
		spec.Machine = "pc-q35-changed"
	},
	"CPU.Mode": func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
		spec.CPU.Mode = hypervisorhelpers.VirtualMachineProfileCPUModeUnknown
	},
	"CPU.Model": func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
		spec.CPU.Model = "changed"
	},
	"CPU.Fallback": func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
		spec.CPU.Fallback = "allow"
	},
	"CPU.Features": func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
		spec.CPU.Features = append(spec.CPU.Features, hypervisor.VirtualMachineProfileCPUFeatureSpec{
			Name:   "pku",
			Policy: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyDisable,
		})
	},
	"Timers.HPET": func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
		spec.Timers.HPET = !spec.Timers.HPET
	},
	"Timers.RTCTickPolicy": func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
		spec.Timers.RTCTickPolicy = hypervisorhelpers.VirtualMachineProfileRTCTickPolicyDelay
	},
	"Timers.PITTickPolicy": func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
		spec.Timers.PITTickPolicy = hypervisorhelpers.VirtualMachineProfilePITTickPolicyDiscard
	},
}

// Every hardware field of a builtin profile is part of its contract: changing any one of them,
// on any builtin, must fail validation.
func TestAMD64ValidateBuiltinContract(t *testing.T) {
	for _, builtin := range profile.Profiles(constants.ArchAmd64) {
		for field, mutate := range builtinContractMutations {
			t.Run(builtin.Metadata().ID()+"/"+field, func(t *testing.T) {
				changed := builtin.TypedSpec().DeepCopy()
				mutate(&changed)
				require.NotEqual(t, *builtin.TypedSpec(), changed, "mutation must change the spec")
				require.Error(t, profile.Validate(constants.ArchAmd64, builtin.Metadata().ID(), changed))
			})
		}
	}
}

// A field added to the profile spec must be added to builtinContractMutations, and so to the
// field-by-field comparison in profile.Validate.
func TestBuiltinContractCoversAllFields(t *testing.T) {
	var fields []string

	var walk func(prefix string, typ reflect.Type)

	walk = func(prefix string, typ reflect.Type) {
		for field := range typ.Fields() {
			path := prefix + field.Name
			if field.Type.Kind() == reflect.Struct {
				walk(path+".", field.Type)

				continue
			}

			fields = append(fields, path)
		}
	}

	walk("", reflect.TypeFor[hypervisor.VirtualMachineProfileSpecSpec]())

	// Builtin selects the validation path rather than describing guest hardware.
	fields = slices.DeleteFunc(fields, func(field string) bool { return field == "Builtin" })

	require.ElementsMatch(t, fields, slices.Collect(maps.Keys(builtinContractMutations)))
}

func TestAMD64ValidateCustom(t *testing.T) {
	valid := hypervisor.VirtualMachineProfileSpecSpec{
		Machine: "pc-q35-9.2",
		CPU: hypervisor.VirtualMachineProfileCPUSpec{
			Mode:     hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
			Model:    "Haswell-noTSX-IBRS",
			Fallback: "forbid",
			Features: []hypervisor.VirtualMachineProfileCPUFeatureSpec{
				{
					Name:   "avx2",
					Policy: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyRequire,
				},
			},
		},
		Timers: hypervisor.DefaultVirtualMachineProfileTimersSpec(),
	}
	require.NoError(t, profile.ValidateCustom(constants.ArchAmd64, "cluster", valid))

	for _, name := range []string{"host", "legacy", "passthrough", "compat-v3", "", "bad name"} {
		t.Run("reserved-or-invalid-name-"+name, func(t *testing.T) {
			require.Error(t, profile.ValidateCustom(constants.ArchAmd64, name, valid))
		})
	}

	for _, test := range []struct {
		name   string
		mutate func(*hypervisor.VirtualMachineProfileSpecSpec)
	}{
		{
			name: "builtin-flag",
			mutate: func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
				spec.Builtin = true
			},
		},
		{
			name: "arm-machine",
			mutate: func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
				spec.Machine = "virt"
			},
		},
		{
			name: "unknown-rtc-tick-policy",
			mutate: func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
				spec.Timers.RTCTickPolicy = hypervisorhelpers.VirtualMachineProfileRTCTickPolicyUnknown
			},
		},
		{
			name: "unknown-pit-tick-policy",
			mutate: func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
				spec.Timers.PITTickPolicy = hypervisorhelpers.VirtualMachineProfilePITTickPolicyUnknown
			},
		},
		{
			name: "invalid-mode",
			mutate: func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
				spec.CPU.Mode = hypervisorhelpers.VirtualMachineProfileCPUMode(-1)
			},
		},
		{
			name: "missing-model",
			mutate: func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
				spec.CPU.Model = ""
			},
		},
		{
			name: "invalid-policy",
			mutate: func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
				spec.CPU.Features[0].Policy = hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicy(-1)
			},
		},
		{
			name: "invalid-fallback",
			mutate: func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
				spec.CPU.Fallback = "invalid"
			},
		},
		{
			name: "duplicate-feature",
			mutate: func(spec *hypervisor.VirtualMachineProfileSpecSpec) {
				spec.CPU.Features = append(spec.CPU.Features, spec.CPU.Features[0])
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			spec := valid
			spec.CPU.Features = append([]hypervisor.VirtualMachineProfileCPUFeatureSpec(nil), valid.CPU.Features...)
			test.mutate(&spec)
			require.Error(t, profile.ValidateCustom(constants.ArchAmd64, "cluster", spec))
		})
	}
}
