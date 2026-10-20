// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build amd64

package profile_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/profile"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestProfiles(t *testing.T) {
	builtins := profile.Profiles()
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
		}, *builtin.TypedSpec())
		require.NoError(t, profile.Validate(expected.name, *builtin.TypedSpec()))
	}

	builtins[0].TypedSpec().Machine = "changed"
	require.Error(t, profile.Validate("host", *builtins[0].TypedSpec()))
	builtins[0].TypedSpec().CPU.Features = []hypervisor.VirtualMachineProfileCPUFeatureSpec{
		{
			Name: "changed",
		},
	}
	fresh := profile.Profiles()
	require.Equal(t, "host", fresh[0].Metadata().ID())
	require.Equal(t, "q35", fresh[0].TypedSpec().Machine)
	require.Empty(t, fresh[0].TypedSpec().CPU.Features)
}

func TestValidateCustom(t *testing.T) {
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
	}
	require.NoError(t, profile.ValidateCustom("cluster", valid))

	for _, name := range []string{"host", "legacy", "passthrough", "compat-v3", "", "bad name"} {
		t.Run("reserved-or-invalid-name-"+name, func(t *testing.T) {
			require.Error(t, profile.ValidateCustom(name, valid))
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
			require.Error(t, profile.ValidateCustom("cluster", spec))
		})
	}
}
