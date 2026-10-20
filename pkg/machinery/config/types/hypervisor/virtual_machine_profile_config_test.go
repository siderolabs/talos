// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

func TestVirtualMachineProfileEnumDocValues(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		doc      *encoder.Doc
		field    string
		expected []string
	}{
		{
			name:     "CPU mode",
			doc:      hypervisor.VirtualMachineProfileCPU{}.Doc(),
			field:    "mode",
			expected: hypervisorhelpers.NameableValues(hypervisorhelpers.VirtualMachineProfileCPUModeStrings()),
		},
		{
			name:     "CPU feature policy",
			doc:      hypervisor.VirtualMachineProfileCPUFeature{}.Doc(),
			field:    "policy",
			expected: hypervisorhelpers.NameableValues(hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyStrings()),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			idx := slices.IndexFunc(test.doc.Fields, func(field encoder.Doc) bool { return field.Name == test.field })
			require.GreaterOrEqual(t, idx, 0, "%s field not found in the document's docs", test.field)
			require.Equal(t, test.expected, test.doc.Fields[idx].Values)
		})
	}
}

func TestVirtualMachineProfileConfig(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes([]byte(`apiVersion: v1alpha1
kind: VirtualMachineProfileConfig
name: rack-a
machine: pc-q35-9.2
cpu:
  mode: custom
  model: x86-64-v3
  features:
    - name: pku
      policy: disable
`))
	require.NoError(t, err)
	require.Len(t, provider.VirtualMachineProfileConfigs(), 1)
	profile := provider.VirtualMachineProfileConfigs()[0]
	require.Equal(t, "rack-a", profile.Name())
	require.Equal(t, "pc-q35-9.2", profile.Machine())
	require.Equal(t, hypervisorhelpers.VirtualMachineProfileCPUModeCustom, profile.CPU().Mode())
	require.Equal(t, "x86-64-v3", profile.CPU().Model())
	require.Equal(t, hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyDisable, profile.CPU().Features()[0].Policy())
}

func TestVirtualMachineProfileEnumRoundTrip(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		mode hypervisorhelpers.VirtualMachineProfileCPUMode
	}{
		{
			name: "host-model",
			mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostModel,
		},
		{
			name: "host-passthrough",
			mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough,
		},
		{
			name: "custom",
			mode: hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := hypervisor.NewVirtualMachineProfileConfigV1Alpha1()
			cfg.MetaName = "rack-a"
			cfg.MachineConfig = "q35"
			cfg.CPUConfig.ModeConfig = tc.mode

			if tc.mode == hypervisorhelpers.VirtualMachineProfileCPUModeCustom {
				cfg.CPUConfig.ModelConfig = "kvm64"
				cfg.CPUConfig.FeaturesConfig = []hypervisor.VirtualMachineProfileCPUFeature{
					{
						NameConfig:   "aes",
						PolicyConfig: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyRequire,
					},
					{
						NameConfig:   "pku",
						PolicyConfig: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyDisable,
					},
					{
						NameConfig:   "avx",
						PolicyConfig: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyOptional,
					},
					{
						NameConfig:   "ssse3",
						PolicyConfig: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyForce,
					},
					{
						NameConfig:   "vmx",
						PolicyConfig: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyForbid,
					},
				}
			}

			encoded, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
			require.NoError(t, err)
			require.Contains(t, string(encoded), "mode: "+tc.name)

			if tc.mode == hypervisorhelpers.VirtualMachineProfileCPUModeCustom {
				for _, policy := range []string{"require", "disable", "optional", "force", "forbid"} {
					require.Contains(t, string(encoded), "policy: "+policy)
				}
			}

			provider, err := configloader.NewFromBytes(encoded)
			require.NoError(t, err)
			require.Len(t, provider.VirtualMachineProfileConfigs(), 1)
			cpu := provider.VirtualMachineProfileConfigs()[0].CPU()
			require.Equal(t, tc.mode, cpu.Mode())
			require.Equal(t, cfg.CPUConfig.ModelConfig, cpu.Model())
			require.Equal(t, cfg.CPUConfig.Fallback(), cpu.Fallback())
			require.Len(t, cpu.Features(), len(cfg.CPUConfig.FeaturesConfig))

			for i, feature := range cpu.Features() {
				require.Equal(t, cfg.CPUConfig.FeaturesConfig[i].NameConfig, feature.Name())
				require.Equal(t, cfg.CPUConfig.FeaturesConfig[i].PolicyConfig, feature.Policy())
			}
		})
	}
}

func TestVirtualMachineProfileFallback(t *testing.T) {
	for _, test := range []struct {
		mode       hypervisorhelpers.VirtualMachineProfileCPUMode
		configured string
		expected   string
	}{
		{
			mode:     hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
			expected: "forbid",
		},
		{
			mode:       hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
			configured: "allow",
			expected:   "allow",
		},
		{
			mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostModel,
		},
		{
			mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough,
		},
		{
			mode:       hypervisorhelpers.VirtualMachineProfileCPUModeHostModel,
			configured: "forbid",
			expected:   "forbid",
		},
	} {
		t.Run(test.mode.String()+"/"+test.configured, func(t *testing.T) {
			cpu := hypervisor.VirtualMachineProfileCPU{
				ModeConfig:     test.mode,
				FallbackConfig: test.configured,
			}
			require.Equal(t, test.expected, cpu.Fallback())
		})
	}
}

func TestVirtualMachineProfileValidation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		mode    hypervisorhelpers.VirtualMachineProfileCPUMode
		model   string
		machine string
		feature string
		policy  hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicy
		want    string
	}{
		{
			name: "host", mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostModel,
			machine: "q35", want: "reserved",
		},
		{
			name: "legacy", mode: hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
			model: "kvm64", machine: "pc-i440fx-9.2", want: "reserved",
		},
		{
			name: "mine", mode: hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
			machine: "q35", want: "cpu.model",
		},
		{
			name: "mine", mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostModel,
			machine: "virt", want: "machine",
		},
		{
			name: "mine", mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough,
			machine: "q35", model: "x86-64-v3", want: "cpu.model",
		},
		{
			name: "mine", mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostModel,
			machine: "q35", feature: "pku", policy: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyDisable,
			want: "cpu.features",
		},
		{
			name: "mine", mode: hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
			model: "x86-64-v3", machine: "q35", feature: "pku",
			policy: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyUnknown,
			want:   "policy",
		},
	} {
		cfg := hypervisor.NewVirtualMachineProfileConfigV1Alpha1()
		cfg.MetaName = tc.name
		cfg.MachineConfig = tc.machine
		cfg.CPUConfig.ModeConfig = tc.mode

		cfg.CPUConfig.ModelConfig = tc.model
		if tc.feature != "" {
			cfg.CPUConfig.FeaturesConfig = []hypervisor.VirtualMachineProfileCPUFeature{{NameConfig: tc.feature, PolicyConfig: tc.policy}}
		}

		_, err := cfg.Validate(validationMode{})
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), tc.want), "%s: %v", tc.name, err)
	}
}
